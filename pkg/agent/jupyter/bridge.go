package jupyter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrSlowJupyterClient = errors.New("jupyter WebSocket client is too slow")
	ErrIOPubRateLimit    = errors.New("jupyter IOPub rate limit exceeded")
)

// ChannelsClient is the browser WebSocket boundary used by the direct bridge.
// Implementations must unblock Read and Write when ctx is canceled.
type ChannelsClient interface {
	Read(ctx context.Context) ([]byte, error)
	Write(ctx context.Context, payload []byte) error
	Close(code int, reason string) error
}

// BridgeConfig bounds one browser-to-kernel channel bridge.
type BridgeConfig struct {
	Limits                Limits
	OutboundQueueMessages int
	OutboundQueueBytes    int64
	IOPubMessagesPerSec   int
	IOPubBytesPerSec      int64
	RateWindow            time.Duration
	ReadinessTimeout      time.Duration
	ReadinessRetry        time.Duration
	HeartbeatInterval     time.Duration
	HeartbeatTimeout      time.Duration
	HeartbeatMisses       int
}

func (c BridgeConfig) normalized() BridgeConfig {
	c.Limits = c.Limits.normalized()
	if c.OutboundQueueMessages <= 0 {
		c.OutboundQueueMessages = 256
	}
	if c.OutboundQueueBytes <= 0 {
		c.OutboundQueueBytes = 32 * 1024 * 1024
	}
	if c.IOPubMessagesPerSec <= 0 {
		c.IOPubMessagesPerSec = 1000
	}
	if c.IOPubBytesPerSec <= 0 {
		c.IOPubBytesPerSec = 1_000_000
	}
	if c.RateWindow <= 0 {
		c.RateWindow = 3 * time.Second
	}
	if c.ReadinessTimeout <= 0 {
		c.ReadinessTimeout = 10 * time.Second
	}
	if c.ReadinessRetry <= 0 {
		c.ReadinessRetry = 500 * time.Millisecond
	}
	if c.HeartbeatInterval <= 0 {
		c.HeartbeatInterval = 10 * time.Second
	}
	if c.HeartbeatTimeout <= 0 {
		c.HeartbeatTimeout = 3 * time.Second
	}
	if c.HeartbeatMisses <= 0 {
		c.HeartbeatMisses = 3
	}
	return c
}

// KernelChannelsBridge connects one browser WebSocket to one immutable kernel
// generation using a dedicated set of ZeroMQ sockets.
type KernelChannelsBridge struct {
	manager *KernelManager
	factory SocketFactory
	config  BridgeConfig
}

// NewKernelChannelsBridge creates a bounded direct channels bridge.
func NewKernelChannelsBridge(manager *KernelManager, factory SocketFactory, config BridgeConfig) (*KernelChannelsBridge, error) {
	if manager == nil {
		return nil, errors.New("jupyter kernel manager is required")
	}
	if factory == nil {
		factory = manager.config.SocketFactory
	}
	if factory == nil {
		return nil, errors.New("jupyter socket factory is required")
	}
	return &KernelChannelsBridge{manager: manager, factory: factory, config: config.normalized()}, nil
}

// Bridge runs until the client disconnects, the kernel generation changes, a
// safety limit is reached, or ctx is canceled. It never stops the kernel.
func (b *KernelChannelsBridge) Bridge(ctx context.Context, kernelID string, client ChannelsClient) error {
	if client == nil {
		return errors.New("jupyter channels client is required")
	}
	addActiveBridge(1)
	defer addActiveBridge(-1)
	connection, release, err := b.manager.Connect(kernelID)
	if err != nil {
		return err
	}
	defer release()

	bridgeCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	generationChanged := make(chan struct{})
	go func() {
		select {
		case <-connection.Context.Done():
			close(generationChanged)
			cancel()
		case <-bridgeCtx.Done():
		}
	}()

	signer, err := NewSigner(connection.Info.Key, connection.Info.SignatureScheme)
	if err != nil {
		return err
	}
	sockets, err := b.openSocketSet(bridgeCtx, connection.Info)
	if err != nil {
		return fmt.Errorf("open Jupyter channel sockets: %w", err)
	}
	defer sockets.Close()

	readinessCtx, readinessCancel := context.WithTimeout(bridgeCtx, b.config.ReadinessTimeout)
	err = b.readinessNudge(readinessCtx, sockets, connection.Info, signer)
	readinessCancel()
	if err != nil {
		_ = client.Close(1013, "Jupyter kernel channels are not ready; retry")
		return fmt.Errorf("jupyter channels readiness nudge: %w", err)
	}

	queue := newOutboundQueue(b.config.OutboundQueueMessages, b.config.OutboundQueueBytes)
	errCh := make(chan error, 8)
	var wg sync.WaitGroup
	start := func(run func() error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := run(); err != nil && !errors.Is(err, context.Canceled) {
				select {
				case errCh <- err:
				default:
				}
				cancel()
			}
		}()
	}

	start(func() error { return b.writeClient(bridgeCtx, client, queue) })
	start(func() error { return b.readClient(bridgeCtx, client, sockets, signer) })
	limiter := newIOPubRateLimiter(b.config)
	for channel, socket := range map[Channel]Socket{
		ChannelShell:   sockets.shell,
		ChannelControl: sockets.control,
		ChannelStdin:   sockets.stdin,
		ChannelIOPub:   sockets.iopub,
	} {
		channel, socket := channel, socket
		start(func() error {
			return b.readKernel(bridgeCtx, kernelID, connection.Generation, channel, socket, signer, queue, limiter)
		})
	}
	start(func() error { return b.monitorHeartbeat(bridgeCtx, sockets.heartbeat, connection.Info) })

	var bridgeErr error
	select {
	case <-bridgeCtx.Done():
		select {
		case <-generationChanged:
			bridgeErr = errors.New("jupyter kernel connection generation changed")
		default:
			bridgeErr = bridgeCtx.Err()
		}
	case bridgeErr = <-errCh:
		cancel()
	}
	sockets.Close()
	queue.Close()
	wg.Wait()

	closeCode, closeReason := 1011, "Jupyter channels bridge closed"
	if bridgeErr == nil || errors.Is(bridgeErr, context.Canceled) {
		closeCode, closeReason = 1000, "Jupyter channels client disconnected"
	} else if errors.Is(bridgeErr, ErrBinaryBuffersUnsupported) {
		closeCode, closeReason = 1003, "unsupported_binary_buffers"
	} else if errors.Is(bridgeErr, ErrSlowJupyterClient) || errors.Is(bridgeErr, ErrIOPubRateLimit) {
		closeCode, closeReason = 1008, bridgeErr.Error()
	} else if isClosed(generationChanged) {
		closeCode, closeReason = 1012, "Jupyter kernel restarted; reconnect"
	}
	closeMetricReason := "internal_error"
	switch {
	case bridgeErr == nil || errors.Is(bridgeErr, context.Canceled):
		closeMetricReason = "client_disconnect"
	case errors.Is(bridgeErr, ErrSlowJupyterClient):
		closeMetricReason = "slow_client"
	case errors.Is(bridgeErr, ErrIOPubRateLimit):
		closeMetricReason = "rate_limit"
	case errors.Is(bridgeErr, ErrBinaryBuffersUnsupported):
		closeMetricReason = "unsupported_binary_buffers"
	case isClosed(generationChanged):
		closeMetricReason = "kernel_restart"
	}
	observeBridgeClose(closeMetricReason)
	_ = client.Close(closeCode, closeReason)
	return bridgeErr
}

type bridgeSocketSet struct {
	shell     Socket
	control   Socket
	stdin     Socket
	iopub     Socket
	heartbeat Socket
	closeOnce sync.Once
}

func (s *bridgeSocketSet) Close() {
	s.closeOnce.Do(func() {
		for _, socket := range []Socket{s.shell, s.control, s.stdin, s.iopub, s.heartbeat} {
			if socket != nil {
				_ = socket.Close()
			}
		}
	})
}

func (b *KernelChannelsBridge) openSocketSet(ctx context.Context, connection ConnectionInfo) (*bridgeSocketSet, error) {
	identity := []byte("runme-" + randomIdentifier())
	set := &bridgeSocketSet{}
	var err error
	if set.shell, err = b.openSocket(ctx, SocketTypeDealer, identity, connection, ChannelShell); err != nil {
		set.Close()
		return nil, err
	}
	if set.control, err = b.openSocket(ctx, SocketTypeDealer, identity, connection, ChannelControl); err != nil {
		set.Close()
		return nil, err
	}
	if set.stdin, err = b.openSocket(ctx, SocketTypeDealer, identity, connection, ChannelStdin); err != nil {
		set.Close()
		return nil, err
	}
	if set.iopub, err = b.openSocket(ctx, SocketTypeSub, nil, connection, ChannelIOPub); err != nil {
		set.Close()
		return nil, err
	}
	if set.heartbeat, err = b.openSocket(ctx, SocketTypeReq, nil, connection, ChannelHeartbeat); err != nil {
		set.Close()
		return nil, err
	}
	return set, nil
}

func (b *KernelChannelsBridge) openSocket(
	ctx context.Context,
	socketType SocketType,
	identity []byte,
	connection ConnectionInfo,
	channel Channel,
) (Socket, error) {
	socket, err := b.factory.NewSocket(ctx, socketType, identity)
	if err != nil {
		return nil, err
	}
	endpoint, err := connection.Endpoint(channel)
	if err != nil {
		_ = socket.Close()
		return nil, err
	}
	if err := socket.Dial(endpoint); err != nil {
		_ = socket.Close()
		return nil, err
	}
	return socket, nil
}

func (b *KernelChannelsBridge) readinessNudge(
	ctx context.Context,
	sockets *bridgeSocketSet,
	connection ConnectionInfo,
	signer *Signer,
) error {
	identity := []byte("runme-nudge-" + randomIdentifier())
	shell, err := b.openSocket(ctx, SocketTypeDealer, identity, connection, ChannelShell)
	if err != nil {
		return err
	}
	defer shell.Close()
	control, err := b.openSocket(ctx, SocketTypeDealer, identity, connection, ChannelControl)
	if err != nil {
		return err
	}
	defer control.Close()

	infoReply := make(chan string, 2)
	iopubMessage := make(chan struct{}, 1)
	errorsCh := make(chan error, 3)
	readInfo := func(socket Socket) {
		for {
			replyFrames, err := socket.ReceiveMultipart()
			if err != nil {
				if ctx.Err() == nil {
					errorsCh <- err
				}
				return
			}
			reply, err := ParseMultipart(replyFrames, signer, b.config.Limits)
			if err != nil {
				errorsCh <- err
				return
			}
			messageType, parentID, err := messageTypeAndParent(reply)
			if err != nil {
				errorsCh <- err
				return
			}
			if messageType == "kernel_info_reply" {
				select {
				case infoReply <- parentID:
				default:
				}
				return
			}
		}
	}
	go readInfo(shell)
	go readInfo(control)
	go func() {
		replyFrames, err := sockets.iopub.ReceiveMultipart()
		if err != nil {
			if ctx.Err() == nil {
				errorsCh <- err
			}
			return
		}
		if _, err := ParseMultipart(replyFrames, signer, b.config.Limits); err != nil {
			errorsCh <- err
			return
		}
		iopubMessage <- struct{}{}
	}()

	ticker := time.NewTicker(b.config.ReadinessRetry)
	defer ticker.Stop()
	gotInfo, gotIOPub := false, false
	sentRequestIDs := make(map[string]struct{})
	send := func() error {
		request, requestID, err := newProtocolRequest("kernel_info_request", `{}`)
		if err != nil {
			return err
		}
		frames, err := MarshalMultipart(request, signer, b.config.Limits)
		if err != nil {
			return err
		}
		sentRequestIDs[requestID] = struct{}{}
		if err := shell.SendMultipart(frames); err != nil {
			return err
		}
		return control.SendMultipart(frames)
	}
	if err := send(); err != nil {
		return err
	}
	for !gotInfo || !gotIOPub {
		select {
		case <-ctx.Done():
			return fmt.Errorf(
				"readiness incomplete (kernel_info=%t iopub=%t): %w",
				gotInfo,
				gotIOPub,
				ctx.Err(),
			)
		case err := <-errorsCh:
			return err
		case parentID := <-infoReply:
			if _, ok := sentRequestIDs[parentID]; ok {
				gotInfo = true
			}
		case <-iopubMessage:
			gotIOPub = true
		case <-ticker.C:
			if err := send(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *KernelChannelsBridge) readClient(
	ctx context.Context,
	client ChannelsClient,
	sockets *bridgeSocketSet,
	signer *Signer,
) error {
	for {
		payload, err := client.Read(ctx)
		if err != nil {
			return err
		}
		channel, message, err := ParseChannelJSON(payload, b.config.Limits)
		if err != nil {
			observeProtocolError(err)
			return err
		}
		frames, err := MarshalMultipart(message, signer, b.config.Limits)
		if err != nil {
			return err
		}
		var socket Socket
		switch channel {
		case ChannelShell:
			socket = sockets.shell
		case ChannelControl:
			socket = sockets.control
		case ChannelStdin:
			socket = sockets.stdin
		default:
			return fmt.Errorf("unsupported browser Jupyter channel %q", channel)
		}
		if err := socket.SendMultipart(frames); err != nil {
			return err
		}
		observeChannelMessage("browser_to_kernel", channel, len(payload))
	}
}

func (b *KernelChannelsBridge) readKernel(
	ctx context.Context,
	kernelID string,
	generation uint64,
	channel Channel,
	socket Socket,
	signer *Signer,
	queue *outboundQueue,
	limiter *iopubRateLimiter,
) error {
	for {
		frames, err := socket.ReceiveMultipart()
		if err != nil {
			return err
		}
		message, err := ParseMultipart(frames, signer, b.config.Limits)
		if err != nil {
			observeProtocolError(err)
			return err
		}
		payload, err := MarshalChannelJSON(channel, message, b.config.Limits)
		if err != nil {
			return err
		}
		if channel == ChannelIOPub {
			if err := limiter.Allow(len(payload)); err != nil {
				addCounter(jupyterRateLimits)
				return err
			}
			b.observeKernelState(kernelID, generation, message)
		}
		if err := queue.Enqueue(ctx, payload); err != nil {
			return err
		}
		observeChannelMessage("kernel_to_browser", channel, len(payload))
	}
}

func (b *KernelChannelsBridge) observeKernelState(kernelID string, generation uint64, message Message) {
	var header struct {
		MessageType string `json:"msg_type"`
	}
	if json.Unmarshal(message.Header, &header) != nil || header.MessageType != "status" {
		return
	}
	var content struct {
		ExecutionState string `json:"execution_state"`
	}
	if json.Unmarshal(message.Content, &content) != nil {
		return
	}
	switch content.ExecutionState {
	case "busy":
		b.manager.SetExecutionState(kernelID, generation, KernelStateBusy)
	case "idle":
		b.manager.SetExecutionState(kernelID, generation, KernelStateIdle)
	}
}

func (b *KernelChannelsBridge) writeClient(ctx context.Context, client ChannelsClient, queue *outboundQueue) error {
	for {
		payload, err := queue.Dequeue(ctx)
		if err != nil {
			return err
		}
		if err := client.Write(ctx, payload); err != nil {
			return err
		}
	}
}

func (b *KernelChannelsBridge) monitorHeartbeat(ctx context.Context, initial Socket, connection ConnectionInfo) error {
	current := initial
	misses := 0
	ticker := time.NewTicker(b.config.HeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
		payload := []byte("runme-heartbeat-" + randomIdentifier())
		if err := current.SendMultipart([][]byte{payload}); err != nil {
			misses++
		} else {
			replyCh := make(chan [][]byte, 1)
			errCh := make(chan error, 1)
			go func(socket Socket) {
				reply, err := socket.ReceiveMultipart()
				if err != nil {
					errCh <- err
					return
				}
				replyCh <- reply
			}(current)
			timer := time.NewTimer(b.config.HeartbeatTimeout)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case reply := <-replyCh:
				timer.Stop()
				if len(reply) == 1 && string(reply[0]) == string(payload) {
					misses = 0
				} else {
					misses++
				}
			case <-errCh:
				timer.Stop()
				misses++
			case <-timer.C:
				misses++
			}
		}
		if misses < b.config.HeartbeatMisses {
			if misses > 0 {
				addCounter(jupyterHeartbeatMisses)
				_ = current.Close()
				replacement, err := b.openSocket(ctx, SocketTypeReq, nil, connection, ChannelHeartbeat)
				if err != nil {
					continue
				}
				current = replacement
				defer current.Close()
			}
			continue
		}
		addCounter(jupyterHeartbeatMisses)
		return fmt.Errorf("jupyter heartbeat missed %d times", misses)
	}
}

func messageTypeAndParent(message Message) (string, string, error) {
	var header, parent struct {
		MessageID   string `json:"msg_id"`
		MessageType string `json:"msg_type"`
	}
	if err := json.Unmarshal(message.Header, &header); err != nil {
		return "", "", err
	}
	if err := json.Unmarshal(message.ParentHeader, &parent); err != nil {
		return "", "", err
	}
	return header.MessageType, parent.MessageID, nil
}

type outboundQueue struct {
	messages chan []byte
	bytes    atomic.Int64
	maxBytes int64
	closed   chan struct{}
	once     sync.Once
}

func newOutboundQueue(maxMessages int, maxBytes int64) *outboundQueue {
	return &outboundQueue{
		messages: make(chan []byte, maxMessages),
		maxBytes: maxBytes,
		closed:   make(chan struct{}),
	}
}

func (q *outboundQueue) Enqueue(ctx context.Context, payload []byte) error {
	size := int64(len(payload))
	if size > q.maxBytes || q.bytes.Add(size) > q.maxBytes {
		q.bytes.Add(-size)
		return ErrSlowJupyterClient
	}
	select {
	case q.messages <- append([]byte(nil), payload...):
		return nil
	case <-ctx.Done():
		q.bytes.Add(-size)
		return ctx.Err()
	case <-q.closed:
		q.bytes.Add(-size)
		return context.Canceled
	default:
		q.bytes.Add(-size)
		return ErrSlowJupyterClient
	}
}

func (q *outboundQueue) Dequeue(ctx context.Context) ([]byte, error) {
	select {
	case payload := <-q.messages:
		q.bytes.Add(-int64(len(payload)))
		return payload, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-q.closed:
		return nil, context.Canceled
	}
}

func (q *outboundQueue) Close() {
	q.once.Do(func() { close(q.closed) })
}

type iopubRateLimiter struct {
	window       time.Duration
	messageLimit int
	byteLimit    int64
	started      time.Time
	messages     int
	bytes        int64
}

func newIOPubRateLimiter(config BridgeConfig) *iopubRateLimiter {
	messageLimit := int(float64(config.IOPubMessagesPerSec) * config.RateWindow.Seconds())
	if messageLimit < 1 {
		messageLimit = 1
	}
	byteLimit := int64(float64(config.IOPubBytesPerSec) * config.RateWindow.Seconds())
	if byteLimit < 1 {
		byteLimit = 1
	}
	return &iopubRateLimiter{
		window:       config.RateWindow,
		messageLimit: messageLimit,
		byteLimit:    byteLimit,
		started:      time.Now(),
	}
}

func (l *iopubRateLimiter) Allow(bytes int) error {
	now := time.Now()
	if now.Sub(l.started) >= l.window {
		l.started = now
		l.messages = 0
		l.bytes = 0
	}
	l.messages++
	l.bytes += int64(bytes)
	if l.messages > l.messageLimit || l.bytes > l.byteLimit {
		return ErrIOPubRateLimit
	}
	return nil
}

func isClosed(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

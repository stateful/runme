package jupyter

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-zeromq/zmq4"
)

// SocketType is one of the three ZeroMQ patterns used by Jupyter clients.
type SocketType string

const (
	SocketTypeDealer SocketType = "dealer"
	SocketTypeSub    SocketType = "sub"
	SocketTypeReq    SocketType = "req"
)

// Socket is the narrow multipart interface used by the kernel manager and
// channels bridge. It intentionally hides the selected ZeroMQ implementation.
type Socket interface {
	Dial(endpoint string) error
	SendMultipart(frames [][]byte) error
	ReceiveMultipart() ([][]byte, error)
	Close() error
}

// SocketFactory creates context-bound Jupyter sockets. DEALER identities are
// supplied by the bridge so shell, control, and stdin can share one identity.
type SocketFactory interface {
	NewSocket(ctx context.Context, socketType SocketType, identity []byte) (Socket, error)
}

// GoZeroMQFactory adapts github.com/go-zeromq/zmq4 to SocketFactory.
type GoZeroMQFactory struct {
	Limits         Limits
	DialTimeout    time.Duration
	DialRetry      time.Duration
	MaxDialRetries int
}

// NewGoZeroMQFactory returns a pure-Go ZeroMQ socket factory with bounded dial
// behavior and multipart limits.
func NewGoZeroMQFactory(limits Limits) *GoZeroMQFactory {
	return &GoZeroMQFactory{
		Limits:         limits.normalized(),
		DialTimeout:    5 * time.Second,
		DialRetry:      100 * time.Millisecond,
		MaxDialRetries: 2,
	}
}

// NewSocket creates a DEALER, subscribe-all SUB, or REQ socket.
func (f *GoZeroMQFactory) NewSocket(ctx context.Context, socketType SocketType, identity []byte) (Socket, error) {
	if ctx == nil {
		return nil, errors.New("jupyter socket context is required")
	}
	if f == nil {
		return nil, errors.New("jupyter socket factory is required")
	}
	if socketType == SocketTypeDealer && len(identity) == 0 {
		return nil, errors.New("jupyter DEALER identity is required")
	}

	dialTimeout := f.DialTimeout
	if dialTimeout <= 0 {
		dialTimeout = 5 * time.Second
	}
	dialRetry := f.DialRetry
	if dialRetry <= 0 {
		dialRetry = 100 * time.Millisecond
	}
	maxDialRetries := f.MaxDialRetries
	if maxDialRetries < 0 {
		return nil, errors.New("unbounded Jupyter dial retries are not allowed")
	}

	opts := []zmq4.Option{
		zmq4.WithDialerTimeout(dialTimeout),
		zmq4.WithDialerRetry(dialRetry),
		zmq4.WithDialerMaxRetries(maxDialRetries),
		zmq4.WithAutomaticReconnect(false),
		zmq4.WithLogger(log.New(io.Discard, "", 0)),
	}
	if len(identity) > 0 {
		opts = append(opts, zmq4.WithID(zmq4.SocketIdentity(identity)))
	}

	var raw zmq4.Socket
	switch socketType {
	case SocketTypeDealer:
		raw = zmq4.NewDealer(ctx, opts...)
	case SocketTypeSub:
		raw = zmq4.NewSub(ctx, opts...)
		if err := raw.SetOption(zmq4.OptionSubscribe, ""); err != nil {
			_ = raw.Close()
			return nil, fmt.Errorf("subscribe to all Jupyter IOPub topics: %w", err)
		}
	case SocketTypeReq:
		raw = zmq4.NewReq(ctx, opts...)
	default:
		return nil, fmt.Errorf("unsupported Jupyter socket type %q", socketType)
	}

	return &goZeroMQSocket{
		raw:    raw,
		limits: f.Limits.normalized(),
	}, nil
}

type goZeroMQSocket struct {
	raw       zmq4.Socket
	limits    Limits
	closed    atomic.Bool
	closeOnce sync.Once
	closeErr  error
}

func (s *goZeroMQSocket) Dial(endpoint string) error {
	if s == nil || s.raw == nil || s.closed.Load() {
		return errors.New("jupyter socket is closed")
	}
	if err := validateLoopbackTCPEndpoint(endpoint); err != nil {
		return err
	}
	if err := s.raw.Dial(endpoint); err != nil {
		return fmt.Errorf("dial Jupyter socket: %w", err)
	}
	return nil
}

func (s *goZeroMQSocket) SendMultipart(frames [][]byte) error {
	if s == nil || s.raw == nil || s.closed.Load() {
		return errors.New("jupyter socket is closed")
	}
	if err := validateMultipartLimits(frames, s.limits); err != nil {
		return err
	}
	if err := s.raw.SendMulti(zmq4.NewMsgFrom(cloneFrames(frames)...)); err != nil {
		return fmt.Errorf("send Jupyter multipart message: %w", err)
	}
	return nil
}

func (s *goZeroMQSocket) ReceiveMultipart() ([][]byte, error) {
	if s == nil || s.raw == nil || s.closed.Load() {
		return nil, errors.New("jupyter socket is closed")
	}
	message, err := s.raw.Recv()
	if err != nil {
		return nil, fmt.Errorf("receive Jupyter multipart message: %w", err)
	}
	if err := message.Err(); err != nil {
		return nil, fmt.Errorf("receive Jupyter multipart message: %w", err)
	}
	if err := validateMultipartLimits(message.Frames, s.limits); err != nil {
		return nil, err
	}
	return cloneFrames(message.Frames), nil
}

func (s *goZeroMQSocket) Close() error {
	if s == nil {
		return nil
	}
	s.closeOnce.Do(func() {
		if s.raw != nil {
			s.closed.Store(true)
			s.closeErr = s.raw.Close()
		}
	})
	return s.closeErr
}

func validateLoopbackTCPEndpoint(endpoint string) error {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("invalid Jupyter endpoint: %w", err)
	}
	if parsed.Scheme != "tcp" {
		return fmt.Errorf("unsupported Jupyter endpoint scheme %q", parsed.Scheme)
	}
	if parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("invalid Jupyter TCP endpoint")
	}
	host, port, err := net.SplitHostPort(parsed.Host)
	if err != nil || strings.TrimSpace(port) == "" {
		return errors.New("invalid Jupyter TCP endpoint address")
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return errors.New("jupyter TCP endpoint must use a loopback IP")
	}
	return nil
}

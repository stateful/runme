package jupyter

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const connectionFilePlaceholder = "{connection_file}"

// KernelState describes the lifecycle of a locally managed kernel.
type KernelState string

const (
	KernelStateStarting     KernelState = "starting"
	KernelStateIdle         KernelState = "idle"
	KernelStateBusy         KernelState = "busy"
	KernelStateInterrupting KernelState = "interrupting"
	KernelStateRestarting   KernelState = "restarting"
	KernelStateStopping     KernelState = "stopping"
	KernelStateDead         KernelState = "dead"
)

// KernelModel is compatible with the Jupyter Server kernel model fields used
// by Runme Web.
type KernelModel struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	LastActivity   string `json:"last_activity"`
	ExecutionState string `json:"execution_state"`
	Connections    int    `json:"connections"`
}

// KernelLaunchSpec describes one client-requested kernel process. Argv follows
// the Jupyter kernelspec convention and must contain exactly one
// {connection_file} placeholder. The server replaces that placeholder with an
// owner-only connection file before launching the process directly, without a
// shell.
type KernelLaunchSpec struct {
	Name string
	Argv []string
}

// PythonKernelLaunchSpec returns the standard ipykernel launch specification.
func PythonKernelLaunchSpec(command string) KernelLaunchSpec {
	if strings.TrimSpace(command) == "" {
		command = "python3"
	}
	return KernelLaunchSpec{
		Name: "python3",
		Argv: []string{command, "-m", "ipykernel_launcher", "-f", connectionFilePlaceholder},
	}
}

// KernelManagerConfig configures local kernel lifecycle boundaries.
type KernelManagerConfig struct {
	RuntimeDir       string
	SocketFactory    SocketFactory
	StartupTimeout   time.Duration
	ReadinessTimeout time.Duration
	GracefulTimeout  time.Duration
	TerminateTimeout time.Duration
	KillTimeout      time.Duration
	DiagnosticsBytes int
}

// KernelManager owns all local kernel processes and immutable connection
// generations independently of HTTP routing.
type KernelManager struct {
	mu      sync.RWMutex
	config  KernelManagerConfig
	kernels map[string]*managedKernel
	ctx     context.Context
	cancel  context.CancelFunc
	closed  bool
}

type managedKernel struct {
	lifecycle    sync.Mutex
	id           string
	name         string
	state        KernelState
	lastActivity time.Time
	connections  int
	generation   uint64
	connection   *kernelConnection
	command      *exec.Cmd
	exited       chan struct{}
	exitErr      error
	runtimeDir   string
	diagnostics  *boundedWriter
	launchSpec   KernelLaunchSpec
}

type kernelConnection struct {
	Generation uint64
	Info       ConnectionInfo
	Context    context.Context
	cancel     context.CancelFunc
}

// NewKernelManager creates a manager and prepares an owner-only runtime
// directory.
func NewKernelManager(config KernelManagerConfig) (*KernelManager, error) {
	if strings.TrimSpace(config.RuntimeDir) == "" {
		return nil, errors.New("jupyter kernel runtime directory is required")
	}
	if config.SocketFactory == nil {
		config.SocketFactory = NewGoZeroMQFactory(Limits{})
	}
	if config.StartupTimeout <= 0 {
		config.StartupTimeout = 15 * time.Second
	}
	if config.ReadinessTimeout <= 0 {
		config.ReadinessTimeout = 10 * time.Second
	}
	if config.GracefulTimeout <= 0 {
		config.GracefulTimeout = 3 * time.Second
	}
	if config.TerminateTimeout <= 0 {
		config.TerminateTimeout = 2 * time.Second
	}
	if config.KillTimeout <= 0 {
		config.KillTimeout = 2 * time.Second
	}
	if config.DiagnosticsBytes <= 0 {
		config.DiagnosticsBytes = 64 * 1024
	}
	if err := os.MkdirAll(config.RuntimeDir, 0o700); err != nil {
		return nil, fmt.Errorf("create Jupyter runtime directory: %w", err)
	}
	if err := os.Chmod(config.RuntimeDir, 0o700); err != nil {
		return nil, fmt.Errorf("secure Jupyter runtime directory: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &KernelManager{
		config:  config,
		kernels: make(map[string]*managedKernel),
		ctx:     ctx,
		cancel:  cancel,
	}, nil
}

func validateKernelLaunchSpec(spec KernelLaunchSpec) error {
	if strings.TrimSpace(spec.Name) == "" {
		return errors.New("jupyter kernel name is required")
	}
	if len(spec.Argv) == 0 {
		return errors.New("jupyter kernel argv is required")
	}
	if len(spec.Argv) > 128 {
		return errors.New("jupyter kernel argv exceeds 128 entries")
	}
	for i, arg := range spec.Argv {
		if strings.IndexByte(arg, 0) >= 0 {
			return fmt.Errorf("jupyter kernel argv[%d] contains a NUL byte", i)
		}
		if len(arg) > 8*1024 {
			return fmt.Errorf("jupyter kernel argv[%d] exceeds 8192 bytes", i)
		}
	}
	if strings.TrimSpace(spec.Argv[0]) == "" {
		return errors.New("jupyter kernel executable is required")
	}
	if strings.Contains(spec.Argv[0], connectionFilePlaceholder) {
		return errors.New("jupyter kernel executable cannot contain the connection-file placeholder")
	}
	placeholderCount := 0
	for _, arg := range spec.Argv[1:] {
		placeholderCount += strings.Count(arg, connectionFilePlaceholder)
	}
	if placeholderCount != 1 {
		return fmt.Errorf("jupyter kernel argv must contain exactly one %s placeholder", connectionFilePlaceholder)
	}
	return nil
}

// Start launches a validated client-requested process and returns only after
// heartbeat and kernel_info readiness checks succeed.
func (m *KernelManager) Start(ctx context.Context, spec KernelLaunchSpec) (KernelModel, error) {
	started := time.Now()
	defer func() { observeLifecycle("start", started) }()
	if err := validateKernelLaunchSpec(spec); err != nil {
		return KernelModel{}, err
	}
	spec.Name = strings.TrimSpace(spec.Name)
	spec.Argv = append([]string(nil), spec.Argv...)
	kernel := &managedKernel{
		id:           uuid.NewString(),
		name:         spec.Name,
		state:        KernelStateStarting,
		lastActivity: time.Now().UTC(),
		launchSpec:   spec,
	}
	kernel.lifecycle.Lock()
	defer kernel.lifecycle.Unlock()

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return KernelModel{}, errors.New("jupyter kernel manager is closed")
	}
	m.kernels[kernel.id] = kernel
	jupyterKernelStates.WithLabelValues(string(kernel.state)).Inc()
	m.mu.Unlock()

	if err := m.launch(ctx, kernel, spec); err != nil {
		_ = m.stopProcess(context.Background(), kernel, false)
		m.mu.Lock()
		jupyterKernelStates.WithLabelValues(string(kernel.state)).Dec()
		delete(m.kernels, kernel.id)
		m.mu.Unlock()
		return KernelModel{}, fmt.Errorf("start Jupyter kernel: %w", err)
	}
	return m.model(kernel.id)
}

// List returns stable snapshots sorted by opaque kernel ID.
func (m *KernelManager) List() []KernelModel {
	m.mu.RLock()
	models := make([]KernelModel, 0, len(m.kernels))
	for _, kernel := range m.kernels {
		models = append(models, kernelModel(kernel))
	}
	m.mu.RUnlock()
	sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
	return models
}

// Get returns one kernel snapshot.
func (m *KernelManager) Get(id string) (KernelModel, error) {
	return m.model(id)
}

func (m *KernelManager) model(id string) (KernelModel, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	kernel, ok := m.kernels[id]
	if !ok {
		return KernelModel{}, fmt.Errorf("jupyter kernel %q not found", id)
	}
	return kernelModel(kernel), nil
}

func kernelModel(kernel *managedKernel) KernelModel {
	return KernelModel{
		ID:             kernel.id,
		Name:           kernel.name,
		LastActivity:   kernel.lastActivity.Format(time.RFC3339Nano),
		ExecutionState: string(kernel.state),
		Connections:    kernel.connections,
	}
}

func (m *KernelManager) launch(ctx context.Context, kernel *managedKernel, spec KernelLaunchSpec) error {
	runtimeDir, err := os.MkdirTemp(m.config.RuntimeDir, "kernel-")
	if err != nil {
		return fmt.Errorf("create kernel runtime directory: %w", err)
	}
	if err := os.Chmod(runtimeDir, 0o700); err != nil {
		_ = os.RemoveAll(runtimeDir)
		return fmt.Errorf("secure kernel runtime directory: %w", err)
	}
	connectionPath := filepath.Join(runtimeDir, "connection.json")
	argv := make([]string, len(spec.Argv))
	for i, arg := range spec.Argv {
		argv[i] = strings.ReplaceAll(arg, connectionFilePlaceholder, connectionPath)
	}
	diagnostics := newBoundedWriter(m.config.DiagnosticsBytes)
	command := exec.Command(argv[0], argv[1:]...)
	command.Stdout = diagnostics
	command.Stderr = diagnostics
	configureManagedProcess(command)
	if err := command.Start(); err != nil {
		_ = os.RemoveAll(runtimeDir)
		return fmt.Errorf("launch kernel %q: %w", spec.Name, err)
	}

	exited := make(chan struct{})
	m.mu.Lock()
	kernel.command = command
	kernel.exited = exited
	kernel.exitErr = nil
	kernel.runtimeDir = runtimeDir
	kernel.diagnostics = diagnostics
	m.setKernelStateLocked(kernel, KernelStateStarting)
	kernel.lastActivity = time.Now().UTC()
	m.mu.Unlock()
	go m.waitForProcess(kernel, command, exited)

	startupCtx, cancel := context.WithTimeout(ctx, m.config.StartupTimeout)
	defer cancel()
	connection, err := waitForManagedConnection(startupCtx, connectionPath, exited)
	if err != nil {
		return fmt.Errorf("wait for connection file: %w; diagnostics: %s", err, diagnostics.String())
	}
	if err := os.Chmod(connectionPath, 0o600); err != nil {
		return fmt.Errorf("secure connection file: %w", err)
	}

	readinessCtx, readinessCancel := context.WithTimeout(startupCtx, m.config.ReadinessTimeout)
	err = probeKernelReadiness(readinessCtx, m.config.SocketFactory, connection)
	readinessCancel()
	if err != nil {
		return fmt.Errorf("kernel readiness failed: %w; diagnostics: %s", err, diagnostics.String())
	}

	generationCtx, generationCancel := context.WithCancel(m.ctx)
	m.mu.Lock()
	if kernel.command != command || kernel.state == KernelStateDead {
		m.mu.Unlock()
		generationCancel()
		return errors.New("kernel exited during readiness")
	}
	kernel.generation++
	kernel.connection = &kernelConnection{
		Generation: kernel.generation,
		Info:       connection,
		Context:    generationCtx,
		cancel:     generationCancel,
	}
	m.setKernelStateLocked(kernel, KernelStateIdle)
	kernel.lastActivity = time.Now().UTC()
	m.mu.Unlock()
	return nil
}

func (m *KernelManager) waitForProcess(kernel *managedKernel, command *exec.Cmd, exited chan struct{}) {
	err := command.Wait()
	m.mu.Lock()
	defer m.mu.Unlock()
	if kernel.command != command {
		close(exited)
		return
	}
	kernel.exitErr = err
	close(exited)
	if kernel.state != KernelStateStopping && kernel.state != KernelStateRestarting {
		m.setKernelStateLocked(kernel, KernelStateDead)
		jupyterUnexpectedExits.Inc()
		kernel.lastActivity = time.Now().UTC()
		if kernel.connection != nil {
			kernel.connection.cancel()
			kernel.connection = nil
		}
	}
}

func waitForManagedConnection(ctx context.Context, path string, exited <-chan struct{}) (ConnectionInfo, error) {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	var lastErr error
	for {
		connection, err := LoadConnectionFile(path)
		if err == nil {
			return connection, nil
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return ConnectionInfo{}, fmt.Errorf("%w (last connection error: %v)", ctx.Err(), lastErr)
		case <-exited:
			return ConnectionInfo{}, fmt.Errorf("kernel process exited (last connection error: %v)", lastErr)
		case <-ticker.C:
		}
	}
}

// Interrupt sends a process-group interrupt without destroying the kernel.
func (m *KernelManager) Interrupt(id string) error {
	started := time.Now()
	defer func() { observeLifecycle("interrupt", started) }()
	m.mu.RLock()
	kernel, ok := m.kernels[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("jupyter kernel %q not found", id)
	}
	kernel.lifecycle.Lock()
	defer kernel.lifecycle.Unlock()

	m.mu.Lock()
	kernel, ok = m.kernels[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("jupyter kernel %q not found", id)
	}
	if kernel.command == nil || kernel.command.Process == nil || kernel.state == KernelStateDead {
		m.mu.Unlock()
		return fmt.Errorf("jupyter kernel %q is not running", id)
	}
	m.setKernelStateLocked(kernel, KernelStateInterrupting)
	kernel.lastActivity = time.Now().UTC()
	command := kernel.command
	m.mu.Unlock()

	if err := interruptManagedProcess(command); err != nil {
		return fmt.Errorf("interrupt Jupyter kernel: %w", err)
	}
	return nil
}

// Restart retains the public kernel ID while replacing its process, ports,
// signing key, and immutable connection generation.
func (m *KernelManager) Restart(ctx context.Context, id string) (KernelModel, error) {
	started := time.Now()
	defer func() { observeLifecycle("restart", started) }()
	m.mu.RLock()
	kernel, ok := m.kernels[id]
	m.mu.RUnlock()
	if !ok {
		return KernelModel{}, fmt.Errorf("jupyter kernel %q not found", id)
	}
	kernel.lifecycle.Lock()
	defer kernel.lifecycle.Unlock()

	m.mu.Lock()
	kernel, ok = m.kernels[id]
	if !ok {
		m.mu.Unlock()
		return KernelModel{}, fmt.Errorf("jupyter kernel %q not found", id)
	}
	spec := kernel.launchSpec
	m.setKernelStateLocked(kernel, KernelStateRestarting)
	kernel.lastActivity = time.Now().UTC()
	if kernel.connection != nil {
		kernel.connection.cancel()
		kernel.connection = nil
	}
	m.mu.Unlock()

	if err := m.stopProcess(ctx, kernel, true); err != nil {
		return KernelModel{}, fmt.Errorf("stop old Jupyter kernel generation: %w", err)
	}
	if err := m.launch(ctx, kernel, spec); err != nil {
		m.mu.Lock()
		m.setKernelStateLocked(kernel, KernelStateDead)
		kernel.lastActivity = time.Now().UTC()
		m.mu.Unlock()
		return KernelModel{}, fmt.Errorf("start new Jupyter kernel generation: %w", err)
	}
	return m.model(id)
}

// Stop shuts down, reaps, and removes a managed kernel.
func (m *KernelManager) Stop(ctx context.Context, id string) error {
	started := time.Now()
	defer func() { observeLifecycle("stop", started) }()
	m.mu.RLock()
	kernel, ok := m.kernels[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("jupyter kernel %q not found", id)
	}
	kernel.lifecycle.Lock()
	defer kernel.lifecycle.Unlock()

	m.mu.Lock()
	kernel, ok = m.kernels[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("jupyter kernel %q not found", id)
	}
	m.setKernelStateLocked(kernel, KernelStateStopping)
	kernel.lastActivity = time.Now().UTC()
	if kernel.connection != nil {
		kernel.connection.cancel()
	}
	m.mu.Unlock()

	if err := m.stopProcess(ctx, kernel, true); err != nil {
		return err
	}
	m.mu.Lock()
	jupyterKernelStates.WithLabelValues(string(kernel.state)).Dec()
	delete(m.kernels, id)
	m.mu.Unlock()
	return nil
}

func (m *KernelManager) stopProcess(ctx context.Context, kernel *managedKernel, graceful bool) error {
	m.mu.RLock()
	command := kernel.command
	exited := kernel.exited
	connection := kernel.connection
	runtimeDir := kernel.runtimeDir
	m.mu.RUnlock()
	if command == nil || command.Process == nil || exited == nil {
		if runtimeDir != "" {
			_ = os.RemoveAll(runtimeDir)
		}
		return nil
	}

	if graceful && connection != nil {
		shutdownCtx, cancel := context.WithTimeout(ctx, m.config.GracefulTimeout)
		_ = requestKernelShutdown(shutdownCtx, m.config.SocketFactory, connection.Info)
		cancel()
	}
	if graceful && waitForExit(exited, m.config.GracefulTimeout) {
		_ = os.RemoveAll(runtimeDir)
		return nil
	}
	select {
	case <-exited:
		_ = os.RemoveAll(runtimeDir)
		return nil
	default:
	}
	if err := terminateManagedProcess(command); err != nil && !isProcessDoneError(err) {
		return fmt.Errorf("terminate Jupyter kernel: %w", err)
	}
	if waitForExit(exited, m.config.TerminateTimeout) {
		_ = os.RemoveAll(runtimeDir)
		return nil
	}
	if err := killManagedProcess(command); err != nil && !isProcessDoneError(err) {
		return fmt.Errorf("kill Jupyter kernel: %w", err)
	}
	if !waitForExit(exited, m.config.KillTimeout) {
		return errors.New("timed out reaping Jupyter kernel after kill")
	}
	_ = os.RemoveAll(runtimeDir)
	return nil
}

func waitForExit(exited <-chan struct{}, timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-exited:
		return true
	case <-timer.C:
		return false
	}
}

// Connect obtains an immutable connection generation and increments the public
// connection count. The release function is idempotent.
func (m *KernelManager) Connect(id string) (kernelConnection, func(), error) {
	m.mu.Lock()
	kernel, ok := m.kernels[id]
	if !ok || kernel.connection == nil || kernel.state == KernelStateDead || kernel.state == KernelStateStopping {
		m.mu.Unlock()
		return kernelConnection{}, nil, fmt.Errorf("jupyter kernel %q is not connectable", id)
	}
	connection := *kernel.connection
	kernel.connections++
	kernel.lastActivity = time.Now().UTC()
	m.mu.Unlock()

	var once sync.Once
	release := func() {
		once.Do(func() {
			m.mu.Lock()
			if current, exists := m.kernels[id]; exists && current.connections > 0 {
				current.connections--
				current.lastActivity = time.Now().UTC()
			}
			m.mu.Unlock()
		})
	}
	return connection, release, nil
}

// SetExecutionState records authenticated IOPub busy/idle transitions for the
// current immutable generation only.
func (m *KernelManager) SetExecutionState(id string, generation uint64, state KernelState) {
	if state != KernelStateBusy && state != KernelStateIdle {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	kernel, ok := m.kernels[id]
	if !ok || kernel.connection == nil || kernel.connection.Generation != generation {
		return
	}
	m.setKernelStateLocked(kernel, state)
	kernel.lastActivity = time.Now().UTC()
}

func (m *KernelManager) setKernelStateLocked(kernel *managedKernel, state KernelState) {
	if kernel.state == state {
		return
	}
	jupyterKernelStates.WithLabelValues(string(kernel.state)).Dec()
	kernel.state = state
	jupyterKernelStates.WithLabelValues(string(kernel.state)).Inc()
}

// Diagnostics returns bounded process output for local troubleshooting.
func (m *KernelManager) Diagnostics(id string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	kernel, ok := m.kernels[id]
	if !ok {
		return "", fmt.Errorf("jupyter kernel %q not found", id)
	}
	if kernel.diagnostics == nil {
		return "", nil
	}
	return kernel.diagnostics.String(), nil
}

// Close stops and reaps every managed kernel. It is safe to call repeatedly.
func (m *KernelManager) Close(ctx context.Context) error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true
	ids := make([]string, 0, len(m.kernels))
	for id := range m.kernels {
		ids = append(ids, id)
	}
	m.mu.Unlock()

	var errs []error
	for _, id := range ids {
		if err := m.Stop(ctx, id); err != nil {
			errs = append(errs, err)
		}
	}
	m.cancel()
	return errors.Join(errs...)
}

func probeKernelReadiness(ctx context.Context, factory SocketFactory, connection ConnectionInfo) error {
	var lastErr error
	for {
		attemptCtx, cancel := context.WithTimeout(ctx, 750*time.Millisecond)
		if err := heartbeatRoundTrip(attemptCtx, factory, connection); err != nil {
			lastErr = err
		} else if err := requestReply(attemptCtx, factory, connection, ChannelControl, "kernel_info_request", "kernel_info_reply", `{}`); err != nil {
			lastErr = err
		} else {
			cancel()
			return nil
		}
		cancel()
		select {
		case <-ctx.Done():
			return fmt.Errorf("jupyter readiness deadline exceeded: %w (last error: %v)", ctx.Err(), lastErr)
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func requestKernelShutdown(ctx context.Context, factory SocketFactory, connection ConnectionInfo) error {
	return requestReply(ctx, factory, connection, ChannelControl, "shutdown_request", "shutdown_reply", `{"restart":false}`)
}

func heartbeatRoundTrip(ctx context.Context, factory SocketFactory, connection ConnectionInfo) error {
	socket, err := factory.NewSocket(ctx, SocketTypeReq, nil)
	if err != nil {
		return err
	}
	defer socket.Close()
	endpoint, err := connection.Endpoint(ChannelHeartbeat)
	if err != nil {
		return err
	}
	if err := socket.Dial(endpoint); err != nil {
		return err
	}
	payload := []byte("runme-ready-" + randomIdentifier())
	if err := socket.SendMultipart([][]byte{payload}); err != nil {
		return err
	}
	reply, err := socket.ReceiveMultipart()
	if err != nil {
		return err
	}
	if len(reply) != 1 || !bytes.Equal(reply[0], payload) {
		return errors.New("unexpected Jupyter heartbeat reply")
	}
	return nil
}

func requestReply(
	ctx context.Context,
	factory SocketFactory,
	connection ConnectionInfo,
	channel Channel,
	requestType, replyType, content string,
) error {
	identity := []byte("runme-" + randomIdentifier())
	socket, err := factory.NewSocket(ctx, SocketTypeDealer, identity)
	if err != nil {
		return err
	}
	defer socket.Close()
	endpoint, err := connection.Endpoint(channel)
	if err != nil {
		return err
	}
	if err := socket.Dial(endpoint); err != nil {
		return err
	}
	signer, err := NewSigner(connection.Key, connection.SignatureScheme)
	if err != nil {
		return err
	}
	request, requestID, err := newProtocolRequest(requestType, content)
	if err != nil {
		return err
	}
	frames, err := MarshalMultipart(request, signer, Limits{})
	if err != nil {
		return err
	}
	if err := socket.SendMultipart(frames); err != nil {
		return err
	}
	replyFrames, err := socket.ReceiveMultipart()
	if err != nil {
		return err
	}
	reply, err := ParseMultipart(replyFrames, signer, Limits{})
	if err != nil {
		return err
	}
	var header, parent struct {
		MessageID   string `json:"msg_id"`
		MessageType string `json:"msg_type"`
	}
	if err := json.Unmarshal(reply.Header, &header); err != nil {
		return err
	}
	if err := json.Unmarshal(reply.ParentHeader, &parent); err != nil {
		return err
	}
	if header.MessageType != replyType || parent.MessageID != requestID {
		return fmt.Errorf("unexpected Jupyter reply type %q or parent %q", header.MessageType, parent.MessageID)
	}
	return nil
}

func newProtocolRequest(messageType, content string) (Message, string, error) {
	messageID := randomIdentifier()
	header, err := json.Marshal(map[string]string{
		"msg_id":   messageID,
		"username": "runme",
		"session":  randomIdentifier(),
		"date":     time.Now().UTC().Format(time.RFC3339Nano),
		"msg_type": messageType,
		"version":  "5.3",
	})
	if err != nil {
		return Message{}, "", err
	}
	message := Message{
		Header:       header,
		ParentHeader: []byte(`{}`),
		Metadata:     []byte(`{}`),
		Content:      []byte(content),
	}
	if err := validateJSONObject(message.Content); err != nil {
		return Message{}, "", err
	}
	return message, messageID, nil
}

func randomIdentifier() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return strings.ReplaceAll(uuid.NewString(), "-", "")
	}
	return hex.EncodeToString(raw)
}

type boundedWriter struct {
	mu       sync.Mutex
	capacity int
	data     []byte
}

func newBoundedWriter(capacity int) *boundedWriter {
	return &boundedWriter{capacity: capacity}
}

func (w *boundedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.data = append(w.data, p...)
	if len(w.data) > w.capacity {
		w.data = append([]byte(nil), w.data[len(w.data)-w.capacity:]...)
	}
	return len(p), nil
}

func (w *boundedWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return strings.TrimSpace(string(w.data))
}

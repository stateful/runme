package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	jupyterbridge "github.com/runmedev/runme/v3/pkg/agent/jupyter"
	"github.com/runmedev/runme/v3/pkg/agent/logs"
)

const (
	jupyterKernelsRoute  = "/v1/jupyter/kernels"
	maxKernelRequestBody = 64 * 1024

	// Keep idle Jupyter channels sockets alive across intermediaries. Long-running
	// cells can be quiet, so transport liveness cannot depend on kernel traffic.
	jupyterChannelsPingInterval     = 20 * time.Second
	jupyterChannelsPongWait         = 60 * time.Second
	jupyterChannelsPingWriteTimeout = 10 * time.Second
)

type jupyterKernelsHandler struct {
	manager  *jupyterbridge.KernelManager
	bridge   *jupyterbridge.KernelChannelsBridge
	upgrader websocket.Upgrader
}

type kernelCreateRequest struct {
	Name string   `json:"name"`
	Argv []string `json:"argv"`
}

func newJupyterKernelsHandler(manager *jupyterbridge.KernelManager) (*jupyterKernelsHandler, error) {
	if manager == nil {
		return nil, errors.New("jupyter kernel manager is required")
	}
	bridge, err := jupyterbridge.NewKernelChannelsBridge(manager, nil, jupyterbridge.BridgeConfig{})
	if err != nil {
		return nil, err
	}
	return &jupyterKernelsHandler{
		manager: manager,
		bridge:  bridge,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(_ *http.Request) bool {
				return true
			},
		},
	}, nil
}

func (h *jupyterKernelsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == jupyterKernelsRoute {
		h.handleCollection(w, r)
		return
	}
	if !strings.HasPrefix(r.URL.Path, jupyterKernelsRoute+"/") {
		writeHTTPError(w, http.StatusNotFound, "not found")
		return
	}

	trimmed := strings.TrimPrefix(r.URL.Path, jupyterKernelsRoute+"/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 1 || len(parts) > 2 || strings.TrimSpace(parts[0]) == "" {
		writeHTTPError(w, http.StatusNotFound, "not found")
		return
	}
	kernelID, err := url.PathUnescape(parts[0])
	if err != nil || strings.TrimSpace(kernelID) == "" || strings.Contains(kernelID, "/") {
		writeHTTPError(w, http.StatusBadRequest, "invalid kernel id")
		return
	}
	if _, err := h.manager.Get(kernelID); err != nil {
		writeHTTPError(w, http.StatusNotFound, "kernel not found")
		return
	}

	if len(parts) == 1 {
		h.handleKernel(w, r, kernelID)
		return
	}
	switch parts[1] {
	case "interrupt":
		h.handleInterrupt(w, r, kernelID)
	case "restart":
		h.handleRestart(w, r, kernelID)
	case "channels":
		h.handleChannels(w, r, kernelID)
	default:
		writeHTTPError(w, http.StatusNotFound, "not found")
	}
}

func (h *jupyterKernelsHandler) handleCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, h.manager.List())
	case http.MethodPost:
		var request kernelCreateRequest
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxKernelRequestBody))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil && !errors.Is(err, io.EOF) {
			writeHTTPError(w, http.StatusBadRequest, "invalid kernel request")
			return
		}
		if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			writeHTTPError(w, http.StatusBadRequest, "invalid kernel request")
			return
		}
		name := strings.TrimSpace(request.Name)
		kernel, err := h.manager.Start(r.Context(), jupyterbridge.KernelLaunchSpec{
			Name: name,
			Argv: request.Argv,
		})
		if err != nil {
			logs.FromContext(r.Context()).Error(err, "failed to start Jupyter kernel", "kernel_name", name)
			writeHTTPError(w, http.StatusBadRequest, "failed to start kernel")
			return
		}
		logs.FromContext(r.Context()).Info("started Jupyter kernel", "kernel_id", kernel.ID, "kernel_name", name)
		writeJSON(w, http.StatusCreated, kernel)
	default:
		writeHTTPError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *jupyterKernelsHandler) handleKernel(w http.ResponseWriter, r *http.Request, kernelID string) {
	switch r.Method {
	case http.MethodGet:
		kernel, err := h.manager.Get(kernelID)
		if err != nil {
			writeHTTPError(w, http.StatusNotFound, "kernel not found")
			return
		}
		writeJSON(w, http.StatusOK, kernel)
	case http.MethodDelete:
		logs.FromContext(r.Context()).Info("stopping Jupyter kernel", "kernel_id", kernelID)
		if err := h.manager.Stop(r.Context(), kernelID); err != nil {
			writeKernelOperationError(w, r, "stop", err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeHTTPError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *jupyterKernelsHandler) handleInterrupt(w http.ResponseWriter, r *http.Request, kernelID string) {
	if r.Method != http.MethodPost {
		writeHTTPError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if err := h.manager.Interrupt(kernelID); err != nil {
		writeKernelOperationError(w, r, "interrupt", err)
		return
	}
	logs.FromContext(r.Context()).Info("interrupted Jupyter kernel", "kernel_id", kernelID)
	w.WriteHeader(http.StatusNoContent)
}

func (h *jupyterKernelsHandler) handleRestart(w http.ResponseWriter, r *http.Request, kernelID string) {
	if r.Method != http.MethodPost {
		writeHTTPError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	kernel, err := h.manager.Restart(r.Context(), kernelID)
	if err != nil {
		writeKernelOperationError(w, r, "restart", err)
		return
	}
	logs.FromContext(r.Context()).Info("restarted Jupyter kernel", "kernel_id", kernelID)
	writeJSON(w, http.StatusOK, kernel)
}

func writeKernelOperationError(w http.ResponseWriter, r *http.Request, operation string, err error) {
	logs.FromContext(r.Context()).Error(err, "Jupyter kernel operation failed", "operation", operation)
	writeHTTPError(w, http.StatusInternalServerError, "kernel operation failed")
}

func (h *jupyterKernelsHandler) handleChannels(w http.ResponseWriter, r *http.Request, kernelID string) {
	if r.Method != http.MethodGet {
		writeHTTPError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	for _, protocol := range websocket.Subprotocols(r) {
		if protocol == "v1.kernel.websocket.jupyter.org" {
			writeHTTPError(w, http.StatusBadRequest, "binary Jupyter WebSocket protocol is not supported")
			return
		}
	}
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		logs.FromContext(r.Context()).Error(err, "failed to upgrade Jupyter channels WebSocket")
		return
	}
	client := newJupyterChannelsClient(conn)
	if err := configureWebSocketPongKeepalive(conn, jupyterChannelsPongWait); err != nil {
		_ = client.Close(websocket.CloseInternalServerErr, "failed to configure WebSocket keepalive")
		return
	}

	keepaliveErr := make(chan error, 1)
	stopKeepalive := make(chan struct{})
	go sendWebSocketKeepalivePings(conn, keepaliveErr, stopKeepalive)
	bridgeErr := h.bridge.Bridge(r.Context(), kernelID, client)
	close(stopKeepalive)
	select {
	case err := <-keepaliveErr:
		if bridgeErr == nil {
			bridgeErr = err
		}
	default:
	}
	if bridgeErr != nil && !errors.Is(bridgeErr, context.Canceled) {
		logs.FromContext(r.Context()).Error(bridgeErr, "Jupyter channels bridge closed", "kernel_id", kernelID)
	}
	_ = client.Close(websocket.CloseNormalClosure, "Jupyter channels closed")
}

type jupyterChannelsClient struct {
	conn      *websocket.Conn
	closeOnce sync.Once
}

func newJupyterChannelsClient(conn *websocket.Conn) *jupyterChannelsClient {
	return &jupyterChannelsClient{conn: conn}
}

func (c *jupyterChannelsClient) Read(ctx context.Context) ([]byte, error) {
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = c.conn.SetReadDeadline(time.Now())
		case <-done:
		}
	}()
	messageType, payload, err := c.conn.ReadMessage()
	close(done)
	if err != nil {
		return nil, err
	}
	if messageType != websocket.TextMessage {
		return nil, jupyterbridge.ErrBinaryBuffersUnsupported
	}
	return payload, nil
}

func (c *jupyterChannelsClient) Write(ctx context.Context, payload []byte) error {
	deadline := time.Now().Add(jupyterChannelsPingWriteTimeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	if err := c.conn.SetWriteDeadline(deadline); err != nil {
		return err
	}
	return c.conn.WriteMessage(websocket.TextMessage, payload)
}

func (c *jupyterChannelsClient) Close(code int, reason string) error {
	var closeErr error
	c.closeOnce.Do(func() {
		closeErr = c.conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(code, reason),
			time.Now().Add(jupyterChannelsPingWriteTimeout),
		)
		if err := c.conn.Close(); closeErr == nil {
			closeErr = err
		}
	})
	return closeErr
}

func configureWebSocketPongKeepalive(conn *websocket.Conn, pongWait time.Duration) error {
	if conn == nil {
		return errors.New("websocket connection is required")
	}
	if pongWait <= 0 {
		return errors.New("pong wait must be positive")
	}
	if err := conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		return err
	}
	conn.SetPongHandler(func(_ string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	return nil
}

func sendWebSocketKeepalivePings(conn *websocket.Conn, errCh chan<- error, stop <-chan struct{}) {
	ticker := time.NewTicker(jupyterChannelsPingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			if err := conn.WriteControl(
				websocket.PingMessage,
				[]byte("runme-keepalive"),
				time.Now().Add(jupyterChannelsPingWriteTimeout),
			); err != nil {
				select {
				case errCh <- fmt.Errorf("write Jupyter channels keepalive: %w", err):
				default:
				}
				return
			}
		}
	}
}

func writeJSON(w http.ResponseWriter, statusCode int, value any) {
	data, err := json.Marshal(value)
	if err != nil {
		writeHTTPError(w, http.StatusInternalServerError, "failed to serialize response")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_, _ = w.Write(data)
}

func writeHTTPError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, map[string]any{"error": message})
}

var _ http.Handler = (*jupyterKernelsHandler)(nil)

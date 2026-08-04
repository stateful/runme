package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	jupyterbridge "github.com/runmedev/runme/v3/pkg/agent/jupyter"
	"github.com/runmedev/runme/v3/pkg/agent/logs"
)

const (
	jupyterServersRoute  = "/v1/jupyter/servers"
	jupyterConfigDir     = "jupyter"
	directZMQProvider    = "direct-zmq"
	maxDirectRequestBody = 64 * 1024

	// Keep idle Jupyter channels sockets alive across intermediaries.
	// Long-running cells can be quiet for extended periods, so liveness cannot
	// depend on a steady stream of Jupyter messages.
	jupyterChannelsPingInterval     = 20 * time.Second
	jupyterChannelsPongWait         = 60 * time.Second
	jupyterChannelsPingWriteTimeout = 10 * time.Second
)

type jupyterProxyHandler struct {
	registry      *jupyterServerRegistry
	directManager *jupyterbridge.KernelManager
	directBridge  *jupyterbridge.KernelChannelsBridge
	httpClient    *http.Client
	upgrader      websocket.Upgrader
}

type directKernelCreateRequest struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func (h *jupyterProxyHandler) handleDirectSubroute(w http.ResponseWriter, r *http.Request, parts []string) {
	switch {
	case len(parts) == 2:
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, h.directManager.List())
		case http.MethodPost:
			var request directKernelCreateRequest
			decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxDirectRequestBody))
			if err := decoder.Decode(&request); err != nil {
				writeHTTPError(w, http.StatusBadRequest, "invalid kernel request")
				return
			}
			if strings.TrimSpace(request.Path) != "" {
				writeHTTPError(w, http.StatusBadRequest, "custom kernel paths are not supported")
				return
			}
			profile := strings.TrimSpace(request.Name)
			if profile == "" {
				profile = "python3"
			}
			kernel, err := h.directManager.Start(r.Context(), profile)
			if err != nil {
				logs.FromContext(r.Context()).Error(err, "failed to start direct Jupyter kernel", "profile", profile)
				writeHTTPError(w, http.StatusBadRequest, "failed to start kernel")
				return
			}
			logs.FromContext(r.Context()).Info("started direct Jupyter kernel", "kernel_id", kernel.ID, "profile", profile)
			writeJSON(w, http.StatusCreated, kernel)
		default:
			writeHTTPError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	case len(parts) == 3:
		kernelSegment, err := url.PathUnescape(parts[2])
		if err != nil || strings.TrimSpace(kernelSegment) == "" {
			writeHTTPError(w, http.StatusBadRequest, "invalid kernel id")
			return
		}
		action := "delete"
		kernelID := kernelSegment
		if strings.HasSuffix(kernelSegment, ":interrupt") {
			action = "interrupt"
			kernelID = strings.TrimSuffix(kernelSegment, ":interrupt")
		} else if strings.HasSuffix(kernelSegment, ":restart") {
			action = "restart"
			kernelID = strings.TrimSuffix(kernelSegment, ":restart")
		}
		if strings.TrimSpace(kernelID) == "" {
			writeHTTPError(w, http.StatusBadRequest, "invalid kernel id")
			return
		}
		if _, err := h.directManager.Get(kernelID); err != nil {
			writeHTTPError(w, http.StatusNotFound, "kernel not found")
			return
		}
		switch action {
		case "interrupt":
			if r.Method != http.MethodPost {
				writeHTTPError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
			if err := h.directManager.Interrupt(kernelID); err != nil {
				writeDirectKernelOperationError(w, r, "interrupt", err)
				return
			}
			logs.FromContext(r.Context()).Info("interrupted direct Jupyter kernel", "kernel_id", kernelID)
			w.WriteHeader(http.StatusNoContent)
		case "restart":
			if r.Method != http.MethodPost {
				writeHTTPError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
			kernel, err := h.directManager.Restart(r.Context(), kernelID)
			if err != nil {
				writeDirectKernelOperationError(w, r, "restart", err)
				return
			}
			logs.FromContext(r.Context()).Info("restarted direct Jupyter kernel", "kernel_id", kernelID)
			writeJSON(w, http.StatusOK, kernel)
		default:
			if r.Method != http.MethodDelete {
				writeHTTPError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
			logs.FromContext(r.Context()).Info("stopping direct Jupyter kernel", "kernel_id", kernelID)
			if err := h.directManager.Stop(r.Context(), kernelID); err != nil {
				writeDirectKernelOperationError(w, r, "stop", err)
				return
			}
			logs.FromContext(r.Context()).Info("stopped direct Jupyter kernel", "kernel_id", kernelID)
			w.WriteHeader(http.StatusNoContent)
		}
		return
	case len(parts) == 4 && parts[3] == "channels":
		if r.Method != http.MethodGet {
			writeHTTPError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		kernelID, err := url.PathUnescape(parts[2])
		if err != nil || strings.TrimSpace(kernelID) == "" {
			writeHTTPError(w, http.StatusBadRequest, "invalid kernel id")
			return
		}
		if _, err := h.directManager.Get(kernelID); err != nil {
			writeHTTPError(w, http.StatusNotFound, "kernel not found")
			return
		}
		h.handleDirectChannels(w, r, kernelID)
		return
	default:
		writeHTTPError(w, http.StatusNotFound, "not found")
	}
}

func writeDirectKernelOperationError(w http.ResponseWriter, r *http.Request, operation string, err error) {
	logs.FromContext(r.Context()).Error(err, "direct Jupyter kernel operation failed", "operation", operation)
	writeHTTPError(w, http.StatusInternalServerError, "kernel operation failed")
}

func (h *jupyterProxyHandler) handleDirectChannels(w http.ResponseWriter, r *http.Request, kernelID string) {
	for _, protocol := range websocket.Subprotocols(r) {
		if protocol == "v1.kernel.websocket.jupyter.org" {
			writeHTTPError(w, http.StatusBadRequest, "binary Jupyter WebSocket protocol is not supported")
			return
		}
	}
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		logs.FromContext(r.Context()).Error(err, "failed to upgrade direct Jupyter channels WebSocket")
		return
	}
	client := newDirectChannelsClient(conn)
	if err := configureWebSocketPongKeepalive(conn, jupyterChannelsPongWait); err != nil {
		_ = client.Close(websocket.CloseInternalServerErr, "failed to configure WebSocket keepalive")
		return
	}

	keepaliveErr := make(chan error, 1)
	stopKeepalive := make(chan struct{})
	go sendWebSocketKeepalivePings(conn, "runme_to_direct_client_keepalive", keepaliveErr, stopKeepalive)
	bridgeErr := h.directBridge.Bridge(r.Context(), kernelID, client)
	close(stopKeepalive)
	select {
	case err := <-keepaliveErr:
		if bridgeErr == nil {
			bridgeErr = err
		}
	default:
	}
	if bridgeErr != nil && !errors.Is(bridgeErr, context.Canceled) {
		logs.FromContext(r.Context()).Error(bridgeErr, "direct Jupyter channels bridge closed", "kernel_id", kernelID)
	}
	_ = client.Close(websocket.CloseNormalClosure, "Jupyter channels closed")
}

type directChannelsClient struct {
	conn      *websocket.Conn
	closeOnce sync.Once
}

func newDirectChannelsClient(conn *websocket.Conn) *directChannelsClient {
	return &directChannelsClient{conn: conn}
}

func (c *directChannelsClient) Read(ctx context.Context) ([]byte, error) {
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

func (c *directChannelsClient) Write(ctx context.Context, payload []byte) error {
	deadline := time.Now().Add(jupyterChannelsPingWriteTimeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	if err := c.conn.SetWriteDeadline(deadline); err != nil {
		return err
	}
	return c.conn.WriteMessage(websocket.TextMessage, payload)
}

func (c *directChannelsClient) Close(code int, reason string) error {
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

func newJupyterProxyHandler(configDir string, managers ...*jupyterbridge.KernelManager) (*jupyterProxyHandler, error) {
	registry, err := newJupyterServerRegistry(configDir)
	if err != nil {
		return nil, err
	}

	var directManager *jupyterbridge.KernelManager
	if len(managers) > 0 {
		directManager = managers[0]
	}
	var directBridge *jupyterbridge.KernelChannelsBridge
	if directManager != nil {
		directBridge, err = jupyterbridge.NewKernelChannelsBridge(directManager, nil, jupyterbridge.BridgeConfig{})
		if err != nil {
			return nil, err
		}
	}
	return &jupyterProxyHandler{
		registry:      registry,
		directManager: directManager,
		directBridge:  directBridge,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(_ *http.Request) bool {
				return true
			},
		},
	}, nil
}

func (h *jupyterProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	switch {
	case path == jupyterServersRoute:
		if r.Method != http.MethodGet {
			writeHTTPError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		h.handleListServers(w, r)
		return
	case strings.HasPrefix(path, jupyterServersRoute+"/"):
		h.handleServerSubroute(w, r)
		return
	default:
		writeHTTPError(w, http.StatusNotFound, "not found")
		return
	}
}

func (h *jupyterProxyHandler) handleListServers(w http.ResponseWriter, r *http.Request) {
	records := h.registry.List()
	if h.directManager != nil {
		records = append(records, jupyterServerPublicRecord{
			Name:         directZMQProvider,
			BaseURL:      "",
			HasToken:     false,
			Backend:      directZMQProvider,
			Capabilities: []string{"kernels", "channels", "interrupt", "restart"},
		})
		sort.Slice(records, func(i, j int) bool { return records[i].Name < records[j].Name })
	}
	writeJSON(w, http.StatusOK, records)
}

func (h *jupyterProxyHandler) handleServerSubroute(w http.ResponseWriter, r *http.Request) {
	trimmed := strings.TrimPrefix(r.URL.Path, jupyterServersRoute+"/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 2 {
		writeHTTPError(w, http.StatusNotFound, "not found")
		return
	}

	serverName, err := url.PathUnescape(parts[0])
	if err != nil || strings.TrimSpace(serverName) == "" {
		writeHTTPError(w, http.StatusBadRequest, "invalid server name")
		return
	}

	if parts[1] != "kernels" {
		writeHTTPError(w, http.StatusNotFound, "not found")
		return
	}
	if serverName == directZMQProvider && h.directManager != nil {
		h.handleDirectSubroute(w, r, parts)
		return
	}

	switch {
	case len(parts) == 2:
		switch r.Method {
		case http.MethodGet, http.MethodPost:
			h.forwardHTTP(w, r, serverName, "/api/kernels")
		default:
			writeHTTPError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	case len(parts) == 3:
		kernelSegment, err := url.PathUnescape(parts[2])
		if err != nil || strings.TrimSpace(kernelSegment) == "" {
			writeHTTPError(w, http.StatusBadRequest, "invalid kernel id")
			return
		}
		switch {
		case strings.HasSuffix(kernelSegment, ":interrupt"):
			if r.Method != http.MethodPost {
				writeHTTPError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
			kernelID := strings.TrimSuffix(kernelSegment, ":interrupt")
			if strings.TrimSpace(kernelID) == "" {
				writeHTTPError(w, http.StatusBadRequest, "invalid kernel id")
				return
			}
			h.forwardHTTP(w, r, serverName, "/api/kernels/"+url.PathEscape(kernelID)+"/interrupt")
			return
		case strings.HasSuffix(kernelSegment, ":restart"):
			if r.Method != http.MethodPost {
				writeHTTPError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
			kernelID := strings.TrimSuffix(kernelSegment, ":restart")
			if strings.TrimSpace(kernelID) == "" {
				writeHTTPError(w, http.StatusBadRequest, "invalid kernel id")
				return
			}
			h.forwardHTTP(w, r, serverName, "/api/kernels/"+url.PathEscape(kernelID)+"/restart")
			return
		default:
			if r.Method != http.MethodDelete {
				writeHTTPError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
			h.forwardHTTP(w, r, serverName, "/api/kernels/"+url.PathEscape(kernelSegment))
			return
		}
	case len(parts) == 4 && parts[3] == "channels":
		kernelID, err := url.PathUnescape(parts[2])
		if err != nil || strings.TrimSpace(kernelID) == "" {
			writeHTTPError(w, http.StatusBadRequest, "invalid kernel id")
			return
		}
		if r.Method != http.MethodGet {
			writeHTTPError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		h.forwardChannelsWebSocket(w, r, serverName, kernelID)
		return
	default:
		writeHTTPError(w, http.StatusNotFound, "not found")
		return
	}
}

func (h *jupyterProxyHandler) forwardHTTP(w http.ResponseWriter, r *http.Request, serverName, upstreamPath string) {
	ctx := r.Context()
	log := logs.FromContext(ctx)

	server, err := h.registry.Resolve(serverName)
	if err != nil {
		writeHTTPError(w, http.StatusNotFound, err.Error())
		return
	}
	if strings.TrimSpace(server.Token) == "" {
		writeHTTPError(w, http.StatusBadRequest, "server is missing jupyter token")
		return
	}

	requestBody, err := readProxyRequestBody(r)
	if err != nil {
		log.Error(err, "failed to read jupyter proxy request body", "server", serverName)
		writeHTTPError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	resp, err := h.doJupyterHTTPRequest(ctx, r, server, upstreamPath, requestBody)
	if err != nil {
		log.Error(err, "Jupyter upstream request failed", "server", serverName)
		writeHTTPError(w, http.StatusBadGateway, "failed to contact jupyter server")
		return
	}

	if resp.StatusCode == http.StatusForbidden {
		refreshedServer, refreshErr := h.registry.ResolveRefreshed(serverName)
		if refreshErr != nil {
			log.Error(refreshErr, "failed to refresh jupyter server config after forbidden response", "server", serverName)
		} else if shouldRetryJupyterRequestWithRefreshedToken(server, refreshedServer) {
			log.Info("retrying jupyter request with refreshed token", "server", serverName, "method", r.Method)
			resp.Body.Close()
			resp, err = h.doJupyterHTTPRequest(ctx, r, refreshedServer, upstreamPath, requestBody)
			if err != nil {
				log.Error(err, "Jupyter upstream retry failed", "server", serverName)
				writeHTTPError(w, http.StatusBadGateway, "failed to contact jupyter server")
				return
			}
		}
	}
	defer resp.Body.Close()

	copyProxyResponseHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Error(err, "failed to copy jupyter response body")
	}
}

func (h *jupyterProxyHandler) doJupyterHTTPRequest(ctx context.Context, r *http.Request, server jupyterServerRecord, upstreamPath string, requestBody []byte) (*http.Response, error) {
	upstreamURL, err := buildUpstreamURL(server.BaseURL, upstreamPath, r.URL.RawQuery)
	if err != nil {
		return nil, errors.New("failed to construct upstream url")
	}
	setUpstreamAuthToken(upstreamURL, server.Token)

	var body io.Reader
	if len(requestBody) > 0 {
		body = bytes.NewReader(requestBody)
	}
	upstreamReq, err := http.NewRequestWithContext(ctx, r.Method, upstreamURL.String(), body)
	if err != nil {
		return nil, errors.New("failed to build upstream request")
	}
	copyProxyRequestHeaders(upstreamReq.Header, r.Header)
	upstreamReq.Header.Set("Authorization", "token "+server.Token)

	resp, err := h.httpClient.Do(upstreamReq)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func readProxyRequestBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	return io.ReadAll(r.Body)
}

func shouldRetryJupyterRequestWithRefreshedToken(oldServer, newServer jupyterServerRecord) bool {
	oldToken := strings.TrimSpace(oldServer.Token)
	newToken := strings.TrimSpace(newServer.Token)
	return newToken != "" && newToken != oldToken
}

func (h *jupyterProxyHandler) forwardChannelsWebSocket(w http.ResponseWriter, r *http.Request, serverName, kernelID string) {
	ctx := r.Context()
	log := logs.FromContext(ctx)

	server, err := h.registry.Resolve(serverName)
	if err != nil {
		writeHTTPError(w, http.StatusNotFound, err.Error())
		return
	}
	if strings.TrimSpace(server.Token) == "" {
		writeHTTPError(w, http.StatusBadRequest, "server is missing jupyter token")
		return
	}

	upstreamURL, err := buildUpstreamURL(
		server.BaseURL,
		"/api/kernels/"+url.PathEscape(kernelID)+"/channels",
		sanitizeClientQueryForUpstream(r.URL.RawQuery),
	)
	if err != nil {
		writeHTTPError(w, http.StatusInternalServerError, "failed to construct upstream url")
		return
	}
	setUpstreamAuthToken(upstreamURL, server.Token)
	switch upstreamURL.Scheme {
	case "http":
		upstreamURL.Scheme = "ws"
	case "https":
		upstreamURL.Scheme = "wss"
	default:
		writeHTTPError(w, http.StatusInternalServerError, "invalid upstream url scheme")
		return
	}

	upstreamHeader := http.Header{}
	copyWebSocketDialHeaders(upstreamHeader, r.Header)
	upstreamHeader.Set("Authorization", "token "+server.Token)

	upstreamConn, upstreamResp, err := websocket.DefaultDialer.DialContext(ctx, upstreamURL.String(), upstreamHeader)
	if err != nil {
		statusCode := http.StatusBadGateway
		if upstreamResp != nil && upstreamResp.StatusCode > 0 {
			statusCode = upstreamResp.StatusCode
		}
		log.Error(err, "failed to connect to jupyter channels websocket", "server", serverName, "kernel_id", kernelID)
		writeHTTPError(w, statusCode, "failed to connect to jupyter kernel channels")
		return
	}
	defer upstreamConn.Close()

	clientConn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error(err, "failed to upgrade client websocket for jupyter channels")
		return
	}
	defer clientConn.Close()

	if err := configureWebSocketPongKeepalive(clientConn, jupyterChannelsPongWait); err != nil {
		log.Error(err, "failed to configure jupyter client websocket keepalive")
	}

	errCh := make(chan error, 3)
	stopKeepalive := make(chan struct{})
	defer close(stopKeepalive)

	go bridgeWebSocketMessagesWithContext(ctx, clientConn, upstreamConn, "upstream_to_client", errCh)
	go bridgeWebSocketMessagesWithContext(ctx, upstreamConn, clientConn, "client_to_upstream", errCh)
	go sendWebSocketKeepalivePings(clientConn, "runme_to_client_keepalive", errCh, stopKeepalive)

	if err := <-errCh; err != nil {
		logJupyterWebSocketBridgeClose(log, serverName, kernelID, err)
	}
}

type jupyterServerRegistry struct {
	configDir string
	mu        sync.RWMutex
	servers   map[string]jupyterServerRecord
}

type jupyterServerRecord struct {
	Name    string
	Runner  string
	BaseURL *url.URL
	Token   string
}

type jupyterServerFile struct {
	BaseURLSnake string `json:"base_url"`
	BaseURLCamel string `json:"baseUrl"`
	Token        string `json:"token"`
	Runner       string `json:"runner"`
}

type jupyterServerPublicRecord struct {
	Name         string   `json:"name"`
	Runner       string   `json:"runner"`
	BaseURL      string   `json:"base_url"`
	HasToken     bool     `json:"has_token"`
	Backend      string   `json:"backend"`
	Capabilities []string `json:"capabilities"`
}

func newJupyterServerRegistry(configDir string) (*jupyterServerRegistry, error) {
	if strings.TrimSpace(configDir) == "" {
		return nil, errors.New("config directory is required")
	}
	registry := &jupyterServerRegistry{
		configDir: filepath.Join(configDir, jupyterConfigDir),
		servers:   make(map[string]jupyterServerRecord),
	}
	if err := registry.reload(); err != nil {
		return nil, err
	}
	return registry, nil
}

func (r *jupyterServerRegistry) List() []jupyterServerPublicRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()

	records := make([]jupyterServerPublicRecord, 0, len(r.servers))
	for _, server := range r.servers {
		records = append(records, jupyterServerPublicRecord{
			Name:         server.Name,
			Runner:       server.Runner,
			BaseURL:      server.BaseURL.String(),
			HasToken:     strings.TrimSpace(server.Token) != "",
			Backend:      "jupyter-server",
			Capabilities: []string{"kernels", "channels", "interrupt", "restart"},
		})
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].Name < records[j].Name
	})
	return records
}

func (r *jupyterServerRegistry) Resolve(name string) (jupyterServerRecord, error) {
	if strings.TrimSpace(name) == "" {
		return jupyterServerRecord{}, errors.New("server name is required")
	}

	r.mu.RLock()
	server, ok := r.servers[name]
	r.mu.RUnlock()
	if ok {
		return server, nil
	}

	if err := r.reload(); err != nil {
		return jupyterServerRecord{}, err
	}

	r.mu.RLock()
	server, ok = r.servers[name]
	r.mu.RUnlock()
	if !ok {
		return jupyterServerRecord{}, fmt.Errorf("jupyter server %q not found", name)
	}
	return server, nil
}

func (r *jupyterServerRegistry) ResolveRefreshed(name string) (jupyterServerRecord, error) {
	if strings.TrimSpace(name) == "" {
		return jupyterServerRecord{}, errors.New("server name is required")
	}
	if err := r.reload(); err != nil {
		return jupyterServerRecord{}, err
	}

	r.mu.RLock()
	server, ok := r.servers[name]
	r.mu.RUnlock()
	if !ok {
		return jupyterServerRecord{}, fmt.Errorf("jupyter server %q not found", name)
	}
	return server, nil
}

func (r *jupyterServerRegistry) reload() error {
	entries, err := os.ReadDir(r.configDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			r.mu.Lock()
			r.servers = make(map[string]jupyterServerRecord)
			r.mu.Unlock()
			return nil
		}
		return err
	}

	next := make(map[string]jupyterServerRecord)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		serverName := strings.TrimSuffix(entry.Name(), ".json")
		fullPath := filepath.Join(r.configDir, entry.Name())
		server, err := loadJupyterServerFile(serverName, fullPath)
		if err != nil {
			zap.L().Warn(
				"Skipping invalid jupyter server config",
				zap.String("path", fullPath),
				zap.Error(err),
			)
			continue
		}
		next[serverName] = server
	}

	r.mu.Lock()
	r.servers = next
	r.mu.Unlock()
	return nil
}

func loadJupyterServerFile(name, path string) (jupyterServerRecord, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return jupyterServerRecord{}, err
	}

	var payload jupyterServerFile
	if err := json.Unmarshal(raw, &payload); err != nil {
		return jupyterServerRecord{}, err
	}

	baseURLRaw := strings.TrimSpace(payload.BaseURLSnake)
	if baseURLRaw == "" {
		baseURLRaw = strings.TrimSpace(payload.BaseURLCamel)
	}
	if baseURLRaw == "" {
		return jupyterServerRecord{}, errors.New("base_url is required")
	}

	parsed, err := url.Parse(baseURLRaw)
	if err != nil {
		return jupyterServerRecord{}, errors.New("base_url is invalid")
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return jupyterServerRecord{}, errors.New("base_url must include scheme and host")
	}
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	if !strings.HasSuffix(parsed.Path, "/") {
		parsed.Path += "/"
	}

	return jupyterServerRecord{
		Name:    name,
		Runner:  strings.TrimSpace(payload.Runner),
		BaseURL: parsed,
		Token:   strings.TrimSpace(payload.Token),
	}, nil
}

func buildUpstreamURL(base *url.URL, upstreamPath, rawQuery string) (*url.URL, error) {
	if base == nil {
		return nil, errors.New("base url is required")
	}
	if strings.TrimSpace(upstreamPath) == "" {
		return nil, errors.New("upstream path is required")
	}
	ref := &url.URL{
		Path:     upstreamPath,
		RawQuery: rawQuery,
	}
	return base.ResolveReference(ref), nil
}

func sanitizeClientQueryForUpstream(rawQuery string) string {
	if strings.TrimSpace(rawQuery) == "" {
		return ""
	}

	queryValues, err := url.ParseQuery(rawQuery)
	if err != nil {
		return rawQuery
	}

	for key := range queryValues {
		if strings.EqualFold(strings.TrimSpace(key), "authorization") {
			delete(queryValues, key)
		}
	}

	return queryValues.Encode()
}

func setUpstreamAuthToken(upstreamURL *url.URL, token string) {
	if upstreamURL == nil {
		return
	}
	trimmedToken := strings.TrimSpace(token)
	if trimmedToken == "" {
		return
	}
	query := upstreamURL.Query()
	query.Set("token", trimmedToken)
	upstreamURL.RawQuery = query.Encode()
}

func copyProxyRequestHeaders(dst, src http.Header) {
	for k, values := range src {
		if isHopByHopHeader(k) || isRestrictedProxyRequestHeader(k) {
			continue
		}
		for _, value := range values {
			dst.Add(k, value)
		}
	}
}

func copyProxyResponseHeaders(dst, src http.Header) {
	for k, values := range src {
		if isHopByHopHeader(k) || isCORSResponseHeader(k) {
			continue
		}
		for _, value := range values {
			dst.Add(k, value)
		}
	}
}

func copyWebSocketDialHeaders(dst, src http.Header) {
	const secWebSocketProtocol = "Sec-WebSocket-Protocol"
	for _, value := range src.Values(secWebSocketProtocol) {
		dst.Add(secWebSocketProtocol, value)
	}
}

func isHopByHopHeader(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "connection",
		"keep-alive",
		"proxy-authenticate",
		"proxy-authorization",
		"te",
		"trailers",
		"transfer-encoding",
		"upgrade":
		return true
	default:
		return false
	}
}

func isCORSResponseHeader(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "access-control-allow-origin",
		"access-control-allow-credentials",
		"access-control-allow-methods",
		"access-control-allow-headers",
		"access-control-expose-headers",
		"access-control-max-age":
		return true
	default:
		return false
	}
}

func isRestrictedProxyRequestHeader(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "authorization",
		"origin",
		"referer",
		"cookie",
		"x-xsrftoken":
		return true
	default:
		return false
	}
}

type webSocketBridgeError struct {
	err         error
	direction   string
	operation   string
	srcRemote   string
	dstRemote   string
	messageType int
}

func (e *webSocketBridgeError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *webSocketBridgeError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func remoteAddr(conn *websocket.Conn) string {
	if conn == nil || conn.UnderlyingConn() == nil {
		return ""
	}
	return conn.UnderlyingConn().RemoteAddr().String()
}

func logJupyterWebSocketBridgeClose(log logr.Logger, serverName, kernelID string, err error) {
	if err == nil {
		return
	}

	fields := []any{
		"server", serverName,
		"kernel_id", kernelID,
		"error", err.Error(),
	}

	var bridgeErr *webSocketBridgeError
	if errors.As(err, &bridgeErr) {
		fields = append(
			fields,
			"direction", bridgeErr.direction,
			"operation", bridgeErr.operation,
			"src_remote_addr", bridgeErr.srcRemote,
			"dst_remote_addr", bridgeErr.dstRemote,
		)
		if bridgeErr.messageType > 0 {
			fields = append(fields, "message_type", bridgeErr.messageType)
		}
	}

	var closeErr *websocket.CloseError
	if errors.As(err, &closeErr) {
		fields = append(
			fields,
			"is_websocket_close", true,
			"close_code", closeErr.Code,
			"close_text", closeErr.Text,
		)
	} else {
		fields = append(fields, "is_websocket_close", false)
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		fields = append(
			fields,
			"is_timeout", netErr.Timeout(),
			"is_temporary", netErr.Temporary(),
		)
	}

	log.Info("jupyter channels websocket bridge closed", fields...)
}

func bridgeWebSocketMessagesWithContext(
	ctx context.Context,
	dst, src *websocket.Conn,
	direction string,
	errCh chan<- error,
) {
	done := make(chan struct{})
	defer close(done)

	go func() {
		select {
		case <-ctx.Done():
			// Force src.ReadMessage to return so the bridge can exit promptly.
			_ = src.SetReadDeadline(time.Now())
		case <-done:
		}
	}()

	for {
		msgType, payload, err := src.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				err = ctx.Err()
			}
			errCh <- &webSocketBridgeError{
				err:       err,
				direction: direction,
				operation: "read",
				srcRemote: remoteAddr(src),
				dstRemote: remoteAddr(dst),
			}
			return
		}
		if err := dst.WriteMessage(msgType, payload); err != nil {
			errCh <- &webSocketBridgeError{
				err:         err,
				direction:   direction,
				operation:   "write",
				srcRemote:   remoteAddr(src),
				dstRemote:   remoteAddr(dst),
				messageType: msgType,
			}
			return
		}
	}
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

func sendWebSocketKeepalivePings(conn *websocket.Conn, direction string, errCh chan<- error, stop <-chan struct{}) {
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
				errCh <- &webSocketBridgeError{
					err:         err,
					direction:   direction,
					operation:   "write_ping",
					srcRemote:   "runme",
					dstRemote:   remoteAddr(conn),
					messageType: websocket.PingMessage,
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
	writeJSON(w, statusCode, map[string]any{
		"error": message,
	})
}

var _ http.Handler = (*jupyterProxyHandler)(nil)

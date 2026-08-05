package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/runmedev/runme/v3/pkg/agent/config"
	jupyterbridge "github.com/runmedev/runme/v3/pkg/agent/jupyter"
)

func newTestJupyterKernelsHandler(t *testing.T) (*jupyterKernelsHandler, *jupyterbridge.KernelManager) {
	t.Helper()
	profile := jupyterbridge.PythonLaunchProfile("python3")
	manager, err := jupyterbridge.NewKernelManager(jupyterbridge.KernelManagerConfig{
		RuntimeDir: t.TempDir(),
		Profiles:   map[string]jupyterbridge.LaunchProfile{profile.Name: profile},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = manager.Close(context.Background()) })
	handler, err := newJupyterKernelsHandler(manager)
	if err != nil {
		t.Fatal(err)
	}
	return handler, manager
}

func TestJupyterKernelsRouteContract(t *testing.T) {
	handler, _ := newTestJupyterKernelsHandler(t)

	tests := []struct {
		name   string
		method string
		path   string
		body   string
		status int
	}{
		{name: "list", method: http.MethodGet, path: jupyterKernelsRoute, status: http.StatusOK},
		{name: "collection method", method: http.MethodDelete, path: jupyterKernelsRoute, status: http.StatusMethodNotAllowed},
		{name: "reject custom path", method: http.MethodPost, path: jupyterKernelsRoute, body: `{"name":"python3","path":"/tmp/connection.json"}`, status: http.StatusBadRequest},
		{name: "reject multiple objects", method: http.MethodPost, path: jupyterKernelsRoute, body: `{ } { }`, status: http.StatusBadRequest},
		{name: "unknown kernel", method: http.MethodGet, path: jupyterKernelsRoute + "/missing", status: http.StatusNotFound},
		{name: "unknown action", method: http.MethodPost, path: jupyterKernelsRoute + "/missing/unknown", status: http.StatusNotFound},
		{name: "old server registry removed", method: http.MethodGet, path: "/v1/jupyter/servers", status: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			handler.ServeHTTP(response, request)
			if response.Code != tt.status {
				t.Fatalf("status = %d, want %d; body: %s", response.Code, tt.status, response.Body.String())
			}
		})
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, jupyterKernelsRoute, nil))
	var kernels []jupyterbridge.KernelModel
	if err := json.Unmarshal(response.Body.Bytes(), &kernels); err != nil {
		t.Fatal(err)
	}
	if len(kernels) != 0 {
		t.Fatalf("kernels = %#v, want empty list", kernels)
	}
}

func TestNewJupyterKernelsHandlerRequiresManager(t *testing.T) {
	if _, err := newJupyterKernelsHandler(nil); err == nil {
		t.Fatal("newJupyterKernelsHandler(nil) succeeded")
	}
}

func TestServerRegistersOnlyKernelScopedJupyterRoutes(t *testing.T) {
	runmeServer, err := NewServer(Options{
		Server: &config.AssistantServerConfig{
			ParserService: true,
		},
		ConfigDir: t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	handler, err := runmeServer.BuildHandler()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if runmeServer.jupyterManager != nil {
			_ = runmeServer.jupyterManager.Close(context.Background())
		}
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	response, err := http.Get(server.URL + jupyterKernelsRoute)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("kernel route status = %d, want 200", response.StatusCode)
	}

	legacyResponse, err := http.Get(server.URL + "/v1/jupyter/servers")
	if err != nil {
		t.Fatal(err)
	}
	legacyResponse.Body.Close()
	if legacyResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("legacy server route status = %d, want 404", legacyResponse.StatusCode)
	}
}

func TestDirectJupyterKernelAPIAndWebSocket(t *testing.T) {
	python := os.Getenv("RUNME_TEST_PYTHON")
	if python == "" {
		var err error
		python, err = exec.LookPath("python3")
		if err != nil || exec.Command(python, "-c", "import ipykernel").Run() != nil {
			t.Skip("ipykernel integration test requires RUNME_TEST_PYTHON or python3 with ipykernel")
		}
	} else if err := exec.Command(python, "-c", "import ipykernel").Run(); err != nil {
		t.Fatalf("RUNME_TEST_PYTHON does not provide ipykernel: %v", err)
	}
	profile := jupyterbridge.PythonLaunchProfile(python)
	manager, err := jupyterbridge.NewKernelManager(jupyterbridge.KernelManagerConfig{
		RuntimeDir:     t.TempDir(),
		Profiles:       map[string]jupyterbridge.LaunchProfile{profile.Name: profile},
		StartupTimeout: 15 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = manager.Close(context.Background()) })
	handler, err := newJupyterKernelsHandler(manager)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	createResponse, err := http.Post(
		server.URL+jupyterKernelsRoute,
		"application/json",
		strings.NewReader(`{"name":"python3"}`),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer createResponse.Body.Close()
	if createResponse.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(createResponse.Body)
		t.Fatalf("create status = %d; body: %s", createResponse.StatusCode, body)
	}
	var kernel jupyterbridge.KernelModel
	if err := json.NewDecoder(createResponse.Body).Decode(&kernel); err != nil {
		t.Fatal(err)
	}

	getResponse, err := http.Get(server.URL + jupyterKernelsRoute + "/" + url.PathEscape(kernel.ID))
	if err != nil {
		t.Fatal(err)
	}
	getResponse.Body.Close()
	if getResponse.StatusCode != http.StatusOK {
		t.Fatalf("get status = %d, want 200", getResponse.StatusCode)
	}

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + jupyterKernelsRoute + "/" + url.PathEscape(kernel.ID) + "/channels"
	binaryDialer := websocket.Dialer{Subprotocols: []string{"v1.kernel.websocket.jupyter.org"}}
	unsupported, response, err := binaryDialer.Dial(wsURL, nil)
	if unsupported != nil {
		unsupported.Close()
	}
	if err == nil || response == nil || response.StatusCode != http.StatusBadRequest {
		t.Fatalf("binary subprotocol dial error = %v, response = %#v; want HTTP 400", err, response)
	}
	response.Body.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetReadDeadline(time.Now().Add(20 * time.Second)); err != nil {
		t.Fatal(err)
	}
	const requestID = "runme-direct-http-test"
	request := map[string]any{
		"channel": "shell",
		"header": map[string]any{
			"msg_id": requestID, "msg_type": "execute_request", "username": "runme", "session": "test-session", "version": "5.3",
		},
		"parent_header": map[string]any{},
		"metadata":      map[string]any{},
		"content": map[string]any{
			"code": "shared_http_value = 21\nshared_http_value * 2", "silent": false, "store_history": true, "user_expressions": map[string]any{}, "allow_stdin": false, "stop_on_error": true,
		},
		"buffers": []any{},
	}
	if err := conn.WriteJSON(request); err != nil {
		t.Fatal(err)
	}
	seenReply, seenResult, seenIdle := false, false, false
	for !(seenReply && seenResult && seenIdle) {
		var message struct {
			Channel      string         `json:"channel"`
			Header       map[string]any `json:"header"`
			ParentHeader map[string]any `json:"parent_header"`
			Content      map[string]any `json:"content"`
		}
		if err := conn.ReadJSON(&message); err != nil {
			t.Fatal(err)
		}
		if message.ParentHeader["msg_id"] != requestID {
			continue
		}
		switch message.Header["msg_type"] {
		case "execute_reply":
			seenReply = message.Channel == "shell" && message.Content["status"] == "ok"
		case "execute_result":
			if data, ok := message.Content["data"].(map[string]any); ok {
				seenResult = data["text/plain"] == "42"
			}
		case "status":
			seenIdle = message.Content["execution_state"] == "idle"
		}
	}
	request["buffers"] = []string{"AA=="}
	request["header"].(map[string]any)["msg_id"] = "runme-binary-buffer-test"
	if err := conn.WriteJSON(request); err != nil {
		t.Fatal(err)
	}
	_, _, err = conn.ReadMessage()
	var closeErr *websocket.CloseError
	if !errors.As(err, &closeErr) || closeErr.Code != websocket.CloseUnsupportedData || closeErr.Text != "unsupported_binary_buffers" {
		t.Fatalf("buffered JSON close error = %v, want code 1003 and unsupported_binary_buffers", err)
	}
	_ = conn.Close()

	restartResponse, err := http.Post(
		server.URL+jupyterKernelsRoute+"/"+url.PathEscape(kernel.ID)+"/restart",
		"application/json",
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	restartResponse.Body.Close()
	if restartResponse.StatusCode != http.StatusOK {
		t.Fatalf("restart status = %d, want 200", restartResponse.StatusCode)
	}

	interruptResponse, err := http.Post(
		server.URL+jupyterKernelsRoute+"/"+url.PathEscape(kernel.ID)+"/interrupt",
		"application/json",
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	interruptResponse.Body.Close()
	if interruptResponse.StatusCode != http.StatusNoContent {
		t.Fatalf("interrupt status = %d, want 204", interruptResponse.StatusCode)
	}

	deleteRequest, err := http.NewRequest(http.MethodDelete, server.URL+jupyterKernelsRoute+"/"+url.PathEscape(kernel.ID), nil)
	if err != nil {
		t.Fatal(err)
	}
	deleteResponse, err := http.DefaultClient.Do(deleteRequest)
	if err != nil {
		t.Fatal(err)
	}
	deleteResponse.Body.Close()
	if deleteResponse.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", deleteResponse.StatusCode)
	}
}

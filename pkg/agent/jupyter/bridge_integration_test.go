package jupyter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

type memoryChannelsClient struct {
	inbound  chan []byte
	outbound chan []byte
	mu       sync.Mutex
	code     int
	reason   string
}

func newMemoryChannelsClient() *memoryChannelsClient {
	return &memoryChannelsClient{
		inbound:  make(chan []byte, 16),
		outbound: make(chan []byte, 256),
	}
}

func (c *memoryChannelsClient) Read(ctx context.Context) ([]byte, error) {
	select {
	case payload := <-c.inbound:
		return append([]byte(nil), payload...), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *memoryChannelsClient) Write(ctx context.Context, payload []byte) error {
	select {
	case c.outbound <- append([]byte(nil), payload...):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *memoryChannelsClient) Close(code int, reason string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.code = code
	c.reason = reason
	return nil
}

func TestKernelChannelsBridgeSharedStateAndTwoClients(t *testing.T) {
	manager := newTestKernelManager(t)
	defer manager.Close(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	model, err := manager.Start(ctx, "python3")
	if err != nil {
		t.Fatal(err)
	}
	bridge := newTestBridge(t, manager)

	clientA, cancelA, resultA := startTestBridge(ctx, bridge, model.ID)
	clientB, cancelB, resultB := startTestBridge(ctx, bridge, model.ID)
	defer stopTestBridge(t, cancelA, resultA)
	defer stopTestBridge(t, cancelB, resultB)

	requestA := randomHex(t, 16)
	requestB := randomHex(t, 16)
	clientA.inbound <- browserExecuteRequest(t, requestA, "shared_bridge_value = 100\nshared_bridge_value + 1")
	result := collectBrowserExecution(t, ctx, clientA, requestA)
	if !result["result:101"] || !result["execute_reply"] || !result["status:idle"] {
		t.Fatalf("client A execution result = %v", result)
	}

	clientB.inbound <- browserExecuteRequest(t, requestB, "shared_bridge_value + 2")
	result = collectBrowserExecution(t, ctx, clientB, requestB)
	if !result["result:102"] || !result["execute_reply"] || !result["status:idle"] {
		t.Fatalf("client B did not observe shared kernel state: %v", result)
	}

	if got, err := manager.Get(model.ID); err != nil || got.ExecutionState != string(KernelStateIdle) || got.Connections != 2 {
		t.Fatalf("kernel model while clients connected = %+v, error = %v", got, err)
	}
}

func TestKernelChannelsBridgeConcurrentExecutionsRouteByParent(t *testing.T) {
	manager := newTestKernelManager(t)
	defer manager.Close(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	model, err := manager.Start(ctx, "python3")
	if err != nil {
		t.Fatal(err)
	}
	bridge := newTestBridge(t, manager)
	clientA, cancelA, resultA := startTestBridge(ctx, bridge, model.ID)
	clientB, cancelB, resultB := startTestBridge(ctx, bridge, model.ID)
	defer stopTestBridge(t, cancelA, resultA)
	defer stopTestBridge(t, cancelB, resultB)

	requestA := randomHex(t, 16)
	requestB := randomHex(t, 16)
	clientA.inbound <- browserExecuteRequest(t, requestA, "'client-a-result'")
	clientB.inbound <- browserExecuteRequest(t, requestB, "'client-b-result'")

	results := make(chan map[string]bool, 2)
	go func() { results <- collectBrowserExecution(t, ctx, clientA, requestA) }()
	go func() { results <- collectBrowserExecution(t, ctx, clientB, requestB) }()
	first, second := <-results, <-results
	if !(first["result:'client-a-result'"] || second["result:'client-a-result'"]) {
		t.Fatalf("client A result missing from routed executions: %v / %v", first, second)
	}
	if !(first["result:'client-b-result'"] || second["result:'client-b-result'"]) {
		t.Fatalf("client B result missing from routed executions: %v / %v", first, second)
	}
}

func TestKernelChannelsBridgeInterruptAndReuse(t *testing.T) {
	manager := newTestKernelManager(t)
	defer manager.Close(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	model, err := manager.Start(ctx, "python3")
	if err != nil {
		t.Fatal(err)
	}
	bridge := newTestBridge(t, manager)
	client, cancelBridge, bridgeResult := startTestBridge(ctx, bridge, model.ID)
	defer stopTestBridge(t, cancelBridge, bridgeResult)

	requestID := randomHex(t, 16)
	client.inbound <- browserExecuteRequest(t, requestID, "import time\ntime.sleep(60)")
	waitForBrowserStatus(t, ctx, client, requestID, "busy")
	if err := manager.Interrupt(model.ID); err != nil {
		t.Fatalf("Interrupt() error = %v", err)
	}
	result := collectBrowserExecution(t, ctx, client, requestID)
	if !result["execute_reply"] || !result["status:idle"] {
		t.Fatalf("interrupted execution did not settle: %v", result)
	}
	waitForKernelState(t, manager, model.ID, KernelStateIdle)

	reuseID := randomHex(t, 16)
	client.inbound <- browserExecuteRequest(t, reuseID, "6 * 7")
	result = collectBrowserExecution(t, ctx, client, reuseID)
	if !result["result:42"] || !result["execute_reply"] || !result["status:idle"] {
		t.Fatalf("kernel was not reusable after interrupt: %v", result)
	}
}

func TestKernelChannelsBridgeRestartClosesOldGeneration(t *testing.T) {
	manager := newTestKernelManager(t)
	defer manager.Close(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	model, err := manager.Start(ctx, "python3")
	if err != nil {
		t.Fatal(err)
	}
	bridge := newTestBridge(t, manager)
	client, cancelBridge, bridgeResult := startTestBridge(ctx, bridge, model.ID)

	requestID := randomHex(t, 16)
	client.inbound <- browserExecuteRequest(t, requestID, "restart_only_value = 9")
	_ = collectBrowserExecution(t, ctx, client, requestID)
	if _, err := manager.Restart(ctx, model.ID); err != nil {
		t.Fatalf("Restart() error = %v", err)
	}

	select {
	case err := <-bridgeResult:
		if err == nil || !strings.Contains(err.Error(), "generation changed") {
			t.Fatalf("old bridge error = %v, want generation change", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("old bridge did not close after restart")
	}
	cancelBridge()
	client.mu.Lock()
	closeCode := client.code
	client.mu.Unlock()
	if closeCode != 1012 {
		t.Fatalf("old bridge close code = %d, want 1012", closeCode)
	}

	newClient, cancelNew, newResult := startTestBridge(ctx, bridge, model.ID)
	defer stopTestBridge(t, cancelNew, newResult)
	newRequestID := randomHex(t, 16)
	newClient.inbound <- browserExecuteRequest(t, newRequestID, "restart_only_value")
	result := collectBrowserExecution(t, ctx, newClient, newRequestID)
	if !result["error:NameError"] || !result["execute_reply"] || !result["status:idle"] {
		t.Fatalf("restart did not reset kernel state: %v", result)
	}
}

func TestKernelChannelsBridgeRichOutputsAndQuietExecution(t *testing.T) {
	manager := newTestKernelManager(t)
	defer manager.Close(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	model, err := manager.Start(ctx, "python3")
	if err != nil {
		t.Fatal(err)
	}
	bridge := newTestBridge(t, manager)
	client, cancelBridge, bridgeResult := startTestBridge(ctx, bridge, model.ID)
	defer stopTestBridge(t, cancelBridge, bridgeResult)

	quietID := randomHex(t, 16)
	client.inbound <- browserExecuteRequest(t, quietID, "import time\ntime.sleep(1)\nprint('quiet-finished')")
	quiet := collectBrowserMessages(t, ctx, client, quietID)
	if !quiet["stdout:quiet-finished"] || !quiet["execute_reply:ok"] || !quiet["status:idle"] {
		t.Fatalf("quiet execution = %v", quiet)
	}

	richID := randomHex(t, 16)
	client.inbound <- browserExecuteRequest(t, richID, strings.Join([]string{
		"import sys",
		"from IPython.display import display",
		"print('stdout-marker')",
		"print('stderr-marker', file=sys.stderr)",
		"display({'text/html': '<b>html-marker</b>'}, raw=True)",
		"display({'image/svg+xml': '<svg><text>svg-marker</text></svg>'}, raw=True)",
		"display({'image/png': 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAAB'}, raw=True)",
		"raise ValueError('error-marker')",
	}, "\n"))
	rich := collectBrowserMessages(t, ctx, client, richID)
	for _, key := range []string{
		"stdout:stdout-marker", "stderr:stderr-marker", "mime:text/html", "mime:image/svg+xml",
		"mime:image/png", "error:ValueError", "execute_reply:error", "status:idle",
	} {
		if !rich[key] {
			t.Fatalf("rich execution missing %q: %v", key, rich)
		}
	}
}

func TestKernelChannelsBridgeStdinAndControlRouting(t *testing.T) {
	manager := newTestKernelManager(t)
	defer manager.Close(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	model, err := manager.Start(ctx, "python3")
	if err != nil {
		t.Fatal(err)
	}
	bridge := newTestBridge(t, manager)
	client, cancelBridge, bridgeResult := startTestBridge(ctx, bridge, model.ID)
	defer stopTestBridge(t, cancelBridge, bridgeResult)

	controlID := randomHex(t, 16)
	client.inbound <- browserProtocolRequest(t, ChannelControl, controlID, "kernel_info_request", map[string]any{})
	for {
		message := readBrowserMessage(t, ctx, client)
		parent := message["parent_header"].(map[string]any)
		if fmt.Sprint(parent["msg_id"]) == controlID && fmt.Sprint(message["msg_type"]) == "kernel_info_reply" {
			break
		}
	}

	executeID := randomHex(t, 16)
	request := browserExecuteRequest(t, executeID, "answer = input('prompt:')\nprint('input=' + answer)")
	var execute map[string]any
	if err := json.Unmarshal(request, &execute); err != nil {
		t.Fatal(err)
	}
	execute["content"].(map[string]any)["allow_stdin"] = true
	request, err = json.Marshal(execute)
	if err != nil {
		t.Fatal(err)
	}
	client.inbound <- request
	seen := make(map[string]bool)
	for !(seen["execute_reply:ok"] && seen["status:idle"]) {
		message := readBrowserMessage(t, ctx, client)
		parent := message["parent_header"].(map[string]any)
		if fmt.Sprint(parent["msg_id"]) != executeID {
			continue
		}
		messageType := fmt.Sprint(message["msg_type"])
		content := message["content"].(map[string]any)
		switch messageType {
		case "input_request":
			header := message["header"].(map[string]any)
			client.inbound <- browserProtocolRequest(t, ChannelStdin, randomHex(t, 16), "input_reply", map[string]any{"value": "forty-two"}, header)
		case "stream":
			if strings.Contains(fmt.Sprint(content["text"]), "input=forty-two") {
				seen["stream"] = true
			}
		case "execute_reply":
			seen["execute_reply:"+fmt.Sprint(content["status"])] = true
		case "status":
			seen["status:"+fmt.Sprint(content["execution_state"])] = true
		}
	}
	if !seen["stream"] {
		t.Fatalf("stdin reply was not routed: %v", seen)
	}
}

func TestKernelChannelsBridgeReconnectSoak(t *testing.T) {
	manager := newTestKernelManager(t)
	defer manager.Close(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	model, err := manager.Start(ctx, "python3")
	if err != nil {
		t.Fatal(err)
	}
	bridge := newTestBridge(t, manager)
	for i := 0; i < 10; i++ {
		client, cancelBridge, bridgeResult := startTestBridge(ctx, bridge, model.ID)
		requestID := randomHex(t, 16)
		client.inbound <- browserExecuteRequest(t, requestID, fmt.Sprintf("%d + 1", i))
		result := collectBrowserExecution(t, ctx, client, requestID)
		if !result[fmt.Sprintf("result:%d", i+1)] || !result["status:idle"] {
			t.Fatalf("soak iteration %d = %v", i, result)
		}
		stopTestBridge(t, cancelBridge, bridgeResult)
	}
	if got, err := manager.Get(model.ID); err != nil || got.Connections != 0 || got.ExecutionState != string(KernelStateIdle) {
		t.Fatalf("kernel after reconnect soak = %+v, error = %v", got, err)
	}
}

func newTestBridge(t *testing.T, manager *KernelManager) *KernelChannelsBridge {
	t.Helper()
	bridge, err := NewKernelChannelsBridge(manager, nil, BridgeConfig{
		ReadinessTimeout:  5 * time.Second,
		ReadinessRetry:    100 * time.Millisecond,
		HeartbeatInterval: 100 * time.Millisecond,
		HeartbeatTimeout:  500 * time.Millisecond,
		HeartbeatMisses:   3,
	})
	if err != nil {
		t.Fatal(err)
	}
	return bridge
}

func startTestBridge(
	parent context.Context,
	bridge *KernelChannelsBridge,
	kernelID string,
) (*memoryChannelsClient, context.CancelFunc, chan error) {
	ctx, cancel := context.WithCancel(parent)
	client := newMemoryChannelsClient()
	result := make(chan error, 1)
	go func() { result <- bridge.Bridge(ctx, kernelID, client) }()
	return client, cancel, result
}

func stopTestBridge(t *testing.T, cancel context.CancelFunc, result <-chan error) {
	t.Helper()
	cancel()
	select {
	case <-result:
	case <-time.After(5 * time.Second):
		t.Error("Jupyter bridge did not stop after cancellation")
	}
}

func browserExecuteRequest(t *testing.T, messageID, code string) []byte {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"channel": "shell",
		"header": map[string]any{
			"msg_id":   messageID,
			"username": "runme-test",
			"session":  randomHex(t, 16),
			"date":     time.Now().UTC().Format(time.RFC3339Nano),
			"msg_type": "execute_request",
			"version":  "5.3",
		},
		"parent_header": map[string]any{},
		"metadata":      map[string]any{"runme_unknown": true},
		"content": map[string]any{
			"code":             code,
			"silent":           false,
			"store_history":    true,
			"user_expressions": map[string]any{},
			"allow_stdin":      false,
			"stop_on_error":    true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func browserProtocolRequest(
	t *testing.T,
	channel Channel,
	messageID, messageType string,
	content map[string]any,
	parents ...map[string]any,
) []byte {
	t.Helper()
	parent := map[string]any{}
	if len(parents) > 0 {
		parent = parents[0]
	}
	payload, err := json.Marshal(map[string]any{
		"channel": channel,
		"header": map[string]any{
			"msg_id": messageID, "username": "runme-test", "session": randomHex(t, 16),
			"date": time.Now().UTC().Format(time.RFC3339Nano), "msg_type": messageType, "version": "5.3",
		},
		"parent_header": parent,
		"metadata":      map[string]any{},
		"content":       content,
	})
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func collectBrowserMessages(t *testing.T, ctx context.Context, client *memoryChannelsClient, requestID string) map[string]bool {
	t.Helper()
	seen := make(map[string]bool)
	for !(strings.HasPrefix(firstMapKey(seen, "execute_reply:"), "execute_reply:") && seen["status:idle"]) {
		message := readBrowserMessage(t, ctx, client)
		parent := message["parent_header"].(map[string]any)
		if fmt.Sprint(parent["msg_id"]) != requestID {
			continue
		}
		messageType := fmt.Sprint(message["msg_type"])
		content := message["content"].(map[string]any)
		switch messageType {
		case "status":
			seen["status:"+fmt.Sprint(content["execution_state"])] = true
		case "stream":
			seen[fmt.Sprint(content["name"])+":"+strings.TrimSpace(fmt.Sprint(content["text"]))] = true
		case "display_data", "execute_result":
			if data, ok := content["data"].(map[string]any); ok {
				for mime := range data {
					seen["mime:"+mime] = true
				}
			}
		case "error":
			seen["error:"+fmt.Sprint(content["ename"])] = true
		case "execute_reply":
			seen["execute_reply:"+fmt.Sprint(content["status"])] = true
		}
	}
	return seen
}

func firstMapKey(values map[string]bool, prefix string) string {
	for key := range values {
		if strings.HasPrefix(key, prefix) {
			return key
		}
	}
	return ""
}

func collectBrowserExecution(
	t *testing.T,
	ctx context.Context,
	client *memoryChannelsClient,
	requestID string,
) map[string]bool {
	t.Helper()
	seen := make(map[string]bool)
	for !(seen["execute_reply"] && seen["status:idle"]) {
		message := readBrowserMessage(t, ctx, client)
		parent := message["parent_header"].(map[string]any)
		if fmt.Sprint(parent["msg_id"]) != requestID {
			continue
		}
		messageType := fmt.Sprint(message["msg_type"])
		content := message["content"].(map[string]any)
		switch messageType {
		case "status":
			seen["status:"+fmt.Sprint(content["execution_state"])] = true
		case "execute_result":
			seen["execute_result"] = true
			if data, ok := content["data"].(map[string]any); ok {
				seen["result:"+fmt.Sprint(data["text/plain"])] = true
			}
		case "error":
			seen["error:"+fmt.Sprint(content["ename"])] = true
		default:
			seen[messageType] = true
		}
	}
	return seen
}

func waitForBrowserStatus(
	t *testing.T,
	ctx context.Context,
	client *memoryChannelsClient,
	requestID, state string,
) {
	t.Helper()
	for {
		message := readBrowserMessage(t, ctx, client)
		parent := message["parent_header"].(map[string]any)
		if fmt.Sprint(parent["msg_id"]) != requestID || fmt.Sprint(message["msg_type"]) != "status" {
			continue
		}
		content := message["content"].(map[string]any)
		if fmt.Sprint(content["execution_state"]) == state {
			return
		}
	}
}

func readBrowserMessage(t *testing.T, ctx context.Context, client *memoryChannelsClient) map[string]any {
	t.Helper()
	select {
	case payload := <-client.outbound:
		var message map[string]any
		if err := json.Unmarshal(payload, &message); err != nil {
			t.Fatalf("decode browser message: %v", err)
		}
		return message
	case <-ctx.Done():
		t.Fatalf("wait for browser message: %v", ctx.Err())
		return nil
	}
}

func TestOutboundQueueAndIOPubRateLimits(t *testing.T) {
	queue := newOutboundQueue(1, 4)
	ctx := context.Background()
	if err := queue.Enqueue(ctx, []byte("1234")); err != nil {
		t.Fatal(err)
	}
	if err := queue.Enqueue(ctx, []byte("1")); !errors.Is(err, ErrSlowJupyterClient) {
		t.Fatalf("Enqueue() error = %v, want slow client", err)
	}
	payload, err := queue.Dequeue(ctx)
	if err != nil || string(payload) != "1234" {
		t.Fatalf("Dequeue() = %q, %v", payload, err)
	}

	limiter := newIOPubRateLimiter(BridgeConfig{
		IOPubMessagesPerSec: 1,
		IOPubBytesPerSec:    4,
		RateWindow:          time.Second,
	}.normalized())
	if err := limiter.Allow(4); err != nil {
		t.Fatal(err)
	}
	if err := limiter.Allow(1); !errors.Is(err, ErrIOPubRateLimit) {
		t.Fatalf("Allow() error = %v, want rate limit", err)
	}
}

package jupyter

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

const integrationTestTimeout = 30 * time.Second

type testHeader struct {
	MessageID   string `json:"msg_id"`
	MessageType string `json:"msg_type"`
}

type receivedMessage struct {
	channel Channel
	wire    Message
	header  testHeader
	parent  testHeader
}

func TestIPyKernelCompatibility(t *testing.T) {
	python := findIPyKernelPython(t)
	ctx, cancel := context.WithTimeout(context.Background(), integrationTestTimeout)
	defer cancel()

	connectionPath := filepath.Join(t.TempDir(), "kernel.json")
	command := exec.CommandContext(ctx, python, "-m", "ipykernel_launcher", "-f", connectionPath)
	command.Stdout = nil
	command.Stderr = nil
	if err := command.Start(); err != nil {
		t.Fatalf("start ipykernel: %v", err)
	}
	processDone := make(chan error, 1)
	go func() {
		processDone <- command.Wait()
	}()
	defer stopTestKernel(t, command, processDone)

	connection := waitForConnectionFile(t, ctx, connectionPath, processDone)
	signer, err := NewSigner(connection.Key, connection.SignatureScheme)
	if err != nil {
		t.Fatal(err)
	}
	factory := NewGoZeroMQFactory(Limits{})
	identity := []byte("runme-test-" + randomHex(t, 8))

	heartbeat := openTestSocket(t, ctx, factory, SocketTypeReq, nil, connection, ChannelHeartbeat)
	control := openTestSocket(t, ctx, factory, SocketTypeDealer, identity, connection, ChannelControl)
	shell := openTestSocket(t, ctx, factory, SocketTypeDealer, identity, connection, ChannelShell)
	iopub := openTestSocket(t, ctx, factory, SocketTypeSub, nil, connection, ChannelIOPub)
	stdin := openTestSocket(t, ctx, factory, SocketTypeDealer, identity, connection, ChannelStdin)
	defer heartbeat.Close()
	defer control.Close()
	defer shell.Close()
	defer iopub.Close()
	defer stdin.Close()

	heartbeatPayload := []byte("runme-heartbeat")
	if err := heartbeat.SendMultipart([][]byte{heartbeatPayload}); err != nil {
		t.Fatalf("send heartbeat: %v", err)
	}
	heartbeatReply, err := heartbeat.ReceiveMultipart()
	if err != nil {
		t.Fatalf("receive heartbeat: %v", err)
	}
	if !reflect.DeepEqual(heartbeatReply, [][]byte{heartbeatPayload}) {
		t.Fatalf("heartbeat reply = %q, want echo %q", heartbeatReply, heartbeatPayload)
	}

	sessionID := randomHex(t, 16)
	infoRequest := newTestRequest(t, "kernel_info_request", sessionID, `{}`)
	infoRequestID := messageID(t, infoRequest)
	sendTestMessage(t, control, signer, infoRequest)
	infoReply := receiveTestMessage(t, control, signer, ChannelControl)
	if infoReply.header.MessageType != "kernel_info_reply" {
		t.Fatalf("control reply type = %q, want kernel_info_reply", infoReply.header.MessageType)
	}
	if infoReply.parent.MessageID != infoRequestID {
		t.Fatalf("control parent msg_id = %q, want %q", infoReply.parent.MessageID, infoRequestID)
	}

	messages := make(chan receivedMessage, 64)
	errorsCh := make(chan error, 2)
	go readTestMessages(ctx, shell, signer, ChannelShell, messages, errorsCh)
	go readTestMessages(ctx, iopub, signer, ChannelIOPub, messages, errorsCh)

	first := executeTestCode(t, ctx, shell, signer, sessionID,
		"shared_value = 41\nprint('runme-stream')\nshared_value + 1", messages, errorsCh)
	for _, messageType := range []string{"status:busy", "execute_input", "stream", "execute_result", "result:42", "execute_reply", "status:idle"} {
		if !first[messageType] {
			t.Errorf("first execution did not observe %s; got %v", messageType, first)
		}
	}

	second := executeTestCode(t, ctx, shell, signer, sessionID,
		"shared_value + 2", messages, errorsCh)
	for _, messageType := range []string{"execute_result", "result:43", "execute_reply", "status:idle"} {
		if !second[messageType] {
			t.Errorf("second execution did not observe %s; got %v", messageType, second)
		}
	}

	shutdown := newTestRequest(t, "shutdown_request", sessionID, `{"restart":false}`)
	sendTestMessage(t, control, signer, shutdown)
	shutdownReply := receiveTestMessage(t, control, signer, ChannelControl)
	if shutdownReply.header.MessageType != "shutdown_reply" {
		t.Fatalf("control reply type = %q, want shutdown_reply", shutdownReply.header.MessageType)
	}

	select {
	case err := <-processDone:
		if err != nil {
			t.Fatalf("ipykernel exited after shutdown with error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ipykernel did not exit after shutdown_request")
	}
}

func findIPyKernelPython(t *testing.T) string {
	t.Helper()
	if configured := strings.TrimSpace(os.Getenv("RUNME_TEST_PYTHON")); configured != "" {
		if err := exec.Command(configured, "-c", "import ipykernel").Run(); err != nil {
			t.Fatalf("RUNME_TEST_PYTHON does not provide ipykernel: %v", err)
		}
		return configured
	}
	python, err := exec.LookPath("python3")
	if err != nil || exec.Command(python, "-c", "import ipykernel").Run() != nil {
		t.Skip("ipykernel integration test requires RUNME_TEST_PYTHON or python3 with ipykernel")
	}
	return python
}

func waitForConnectionFile(t *testing.T, ctx context.Context, path string, processDone <-chan error) ConnectionInfo {
	t.Helper()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	var lastErr error
	for {
		connection, err := LoadConnectionFile(path)
		if err == nil {
			return connection
		}
		lastErr = err
		select {
		case <-ctx.Done():
			t.Fatalf("wait for ipykernel connection file: %v (last error: %v)", ctx.Err(), lastErr)
		case err := <-processDone:
			t.Fatalf("ipykernel exited before connection file was ready: %v (last error: %v)", err, lastErr)
		case <-ticker.C:
		}
	}
}

func openTestSocket(
	t *testing.T,
	ctx context.Context,
	factory SocketFactory,
	socketType SocketType,
	identity []byte,
	connection ConnectionInfo,
	channel Channel,
) Socket {
	t.Helper()
	socket, err := factory.NewSocket(ctx, socketType, identity)
	if err != nil {
		t.Fatalf("create %s socket: %v", channel, err)
	}
	endpoint, err := connection.Endpoint(channel)
	if err != nil {
		_ = socket.Close()
		t.Fatalf("resolve %s endpoint: %v", channel, err)
	}
	if err := socket.Dial(endpoint); err != nil {
		_ = socket.Close()
		t.Fatalf("dial %s endpoint: %v", channel, err)
	}
	return socket
}

func newTestRequest(t *testing.T, messageType, sessionID, content string) Message {
	t.Helper()
	header, err := json.Marshal(map[string]string{
		"msg_id":   randomHex(t, 16),
		"username": "runme-test",
		"session":  sessionID,
		"date":     time.Now().UTC().Format(time.RFC3339Nano),
		"msg_type": messageType,
		"version":  "5.3",
	})
	if err != nil {
		t.Fatal(err)
	}
	return Message{
		Header:       header,
		ParentHeader: []byte(`{}`),
		Metadata:     []byte(`{}`),
		Content:      []byte(content),
	}
}

func messageID(t *testing.T, message Message) string {
	t.Helper()
	var header testHeader
	if err := json.Unmarshal(message.Header, &header); err != nil {
		t.Fatal(err)
	}
	return header.MessageID
}

func sendTestMessage(t *testing.T, socket Socket, signer *Signer, message Message) {
	t.Helper()
	frames, err := MarshalMultipart(message, signer, Limits{})
	if err != nil {
		t.Fatalf("marshal Jupyter message: %v", err)
	}
	if err := socket.SendMultipart(frames); err != nil {
		t.Fatalf("send Jupyter message: %v", err)
	}
}

func receiveTestMessage(t *testing.T, socket Socket, signer *Signer, channel Channel) receivedMessage {
	t.Helper()
	frames, err := socket.ReceiveMultipart()
	if err != nil {
		t.Fatalf("receive %s message: %v", channel, err)
	}
	message, err := decodeTestMessage(frames, signer, channel)
	if err != nil {
		t.Fatalf("decode %s message: %v", channel, err)
	}
	return message
}

func decodeTestMessage(frames [][]byte, signer *Signer, channel Channel) (receivedMessage, error) {
	wire, err := ParseMultipart(frames, signer, Limits{})
	if err != nil {
		return receivedMessage{}, err
	}
	var header, parent testHeader
	if err := json.Unmarshal(wire.Header, &header); err != nil {
		return receivedMessage{}, fmt.Errorf("parse header: %w", err)
	}
	if err := json.Unmarshal(wire.ParentHeader, &parent); err != nil {
		return receivedMessage{}, fmt.Errorf("parse parent header: %w", err)
	}
	return receivedMessage{channel: channel, wire: wire, header: header, parent: parent}, nil
}

func readTestMessages(
	ctx context.Context,
	socket Socket,
	signer *Signer,
	channel Channel,
	messages chan<- receivedMessage,
	errorsCh chan<- error,
) {
	for {
		frames, err := socket.ReceiveMultipart()
		if err != nil {
			if ctx.Err() == nil {
				select {
				case errorsCh <- fmt.Errorf("receive %s: %w", channel, err):
				case <-ctx.Done():
				}
			}
			return
		}
		message, err := decodeTestMessage(frames, signer, channel)
		if err != nil {
			select {
			case errorsCh <- fmt.Errorf("decode %s: %w", channel, err):
			case <-ctx.Done():
			}
			return
		}
		select {
		case messages <- message:
		case <-ctx.Done():
			return
		}
	}
}

func executeTestCode(
	t *testing.T,
	ctx context.Context,
	shell Socket,
	signer *Signer,
	sessionID, code string,
	messages <-chan receivedMessage,
	errorsCh <-chan error,
) map[string]bool {
	t.Helper()
	content, err := json.Marshal(map[string]any{
		"code":             code,
		"silent":           false,
		"store_history":    true,
		"user_expressions": map[string]any{},
		"allow_stdin":      false,
		"stop_on_error":    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := newTestRequest(t, "execute_request", sessionID, string(content))
	requestID := messageID(t, request)
	sendTestMessage(t, shell, signer, request)

	seen := make(map[string]bool)
	for !(seen["execute_reply"] && seen["status:idle"]) {
		select {
		case <-ctx.Done():
			t.Fatalf("wait for execution %s: %v; seen %v", requestID, ctx.Err(), seen)
		case err := <-errorsCh:
			t.Fatalf("read kernel messages: %v; seen %v", err, seen)
		case message := <-messages:
			if message.parent.MessageID != requestID {
				continue
			}
			switch message.header.MessageType {
			case "status":
				var content struct {
					ExecutionState string `json:"execution_state"`
				}
				if err := json.Unmarshal(message.wire.Content, &content); err != nil {
					t.Fatalf("parse status content: %v", err)
				}
				seen["status:"+content.ExecutionState] = true
			case "stream":
				var content struct {
					Text string `json:"text"`
				}
				if err := json.Unmarshal(message.wire.Content, &content); err != nil {
					t.Fatalf("parse stream content: %v", err)
				}
				if strings.Contains(content.Text, "runme-stream") {
					seen["stream"] = true
				}
			case "execute_result":
				var content struct {
					Data map[string]any `json:"data"`
				}
				if err := json.Unmarshal(message.wire.Content, &content); err != nil {
					t.Fatalf("parse execute_result content: %v", err)
				}
				if _, ok := content.Data["text/plain"]; ok {
					seen["execute_result"] = true
					seen["result:"+fmt.Sprint(content.Data["text/plain"])] = true
				}
			default:
				seen[message.header.MessageType] = true
			}
		}
	}
	return seen
}

func randomHex(t *testing.T, bytesCount int) string {
	t.Helper()
	raw := make([]byte, bytesCount)
	if _, err := rand.Read(raw); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(raw)
}

func stopTestKernel(t *testing.T, command *exec.Cmd, processDone <-chan error) {
	t.Helper()
	if command.ProcessState != nil && command.ProcessState.Exited() {
		return
	}
	select {
	case <-processDone:
		return
	default:
	}
	if command.Process == nil {
		return
	}
	if err := command.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		t.Errorf("kill test ipykernel: %v", err)
	}
	select {
	case <-processDone:
	case <-time.After(5 * time.Second):
		if runtime.GOOS != "windows" {
			t.Error("test ipykernel was not reaped after kill")
		}
	}
}

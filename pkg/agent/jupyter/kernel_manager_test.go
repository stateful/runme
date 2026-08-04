package jupyter

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestKernelManagerStartRestartStop(t *testing.T) {
	manager := newTestKernelManager(t)
	defer manager.Close(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	started, err := manager.Start(ctx, "python3")
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if started.ExecutionState != string(KernelStateIdle) {
		t.Fatalf("Start() state = %q, want idle", started.ExecutionState)
	}
	if started.ID == "" || started.Name != "python3" {
		t.Fatalf("Start() model = %+v", started)
	}

	firstConnection, release, err := manager.Connect(started.ID)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	if got, _ := manager.Get(started.ID); got.Connections != 1 {
		t.Fatalf("connections = %d, want 1", got.Connections)
	}
	release()
	release()
	if got, _ := manager.Get(started.ID); got.Connections != 0 {
		t.Fatalf("connections after idempotent release = %d, want 0", got.Connections)
	}

	firstCommand, firstRuntimeDir := testKernelProcess(manager, started.ID)
	assertOwnerOnlyPath(t, firstRuntimeDir, 0o700)
	assertOwnerOnlyPath(t, filepath.Join(firstRuntimeDir, "connection.json"), 0o600)

	restarted, err := manager.Restart(ctx, started.ID)
	if err != nil {
		t.Fatalf("Restart() error = %v", err)
	}
	if restarted.ID != started.ID || restarted.ExecutionState != string(KernelStateIdle) {
		t.Fatalf("Restart() model = %+v, want retained idle kernel", restarted)
	}
	if firstCommand.ProcessState == nil {
		t.Fatal("old kernel process was not reaped during restart")
	}
	select {
	case <-firstConnection.Context.Done():
	default:
		t.Fatal("old connection generation was not canceled during restart")
	}
	if _, err := os.Stat(firstRuntimeDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old runtime directory still exists or stat failed unexpectedly: %v", err)
	}

	secondConnection, releaseSecond, err := manager.Connect(started.ID)
	if err != nil {
		t.Fatalf("Connect() after restart error = %v", err)
	}
	defer releaseSecond()
	if secondConnection.Generation <= firstConnection.Generation {
		t.Fatalf("generation = %d, want greater than %d", secondConnection.Generation, firstConnection.Generation)
	}
	if secondConnection.Info == firstConnection.Info {
		t.Fatal("restart reused old ports and signing key")
	}
	secondCommand, secondRuntimeDir := testKernelProcess(manager, started.ID)
	if secondRuntimeDir == firstRuntimeDir {
		t.Fatal("restart reused old runtime directory")
	}

	if err := manager.Stop(ctx, started.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if len(manager.List()) != 0 {
		t.Fatalf("List() after stop = %+v, want empty", manager.List())
	}
	if secondCommand.ProcessState == nil {
		t.Fatal("restarted kernel process was not reaped during stop")
	}
	if _, err := os.Stat(secondRuntimeDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("runtime directory still exists after stop or stat failed unexpectedly: %v", err)
	}
}

func TestKernelManagerUnexpectedExitBecomesDead(t *testing.T) {
	manager := newTestKernelManager(t)
	defer manager.Close(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	model, err := manager.Start(ctx, "python3")
	if err != nil {
		t.Fatal(err)
	}
	connection, release, err := manager.Connect(model.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	command, _ := testKernelProcess(manager, model.ID)
	if err := killManagedProcess(command); err != nil {
		t.Fatalf("killManagedProcess() error = %v", err)
	}

	waitForKernelState(t, manager, model.ID, KernelStateDead)
	select {
	case <-connection.Context.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("connection generation remained live after process crash")
	}
	if _, _, err := manager.Connect(model.ID); err == nil {
		t.Fatal("Connect() accepted a dead kernel")
	}
	if err := manager.Stop(ctx, model.ID); err != nil {
		t.Fatalf("Stop() dead kernel error = %v", err)
	}
}

func TestKernelManagerCloseReapsAllKernels(t *testing.T) {
	manager := newTestKernelManager(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	var commands []*exec.Cmd
	var runtimeDirs []string
	for range 2 {
		model, err := manager.Start(ctx, "python3")
		if err != nil {
			t.Fatal(err)
		}
		command, runtimeDir := testKernelProcess(manager, model.ID)
		commands = append(commands, command)
		runtimeDirs = append(runtimeDirs, runtimeDir)
	}
	if err := manager.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if len(manager.List()) != 0 {
		t.Fatalf("List() after Close() = %+v, want empty", manager.List())
	}
	for i, command := range commands {
		if command.ProcessState == nil {
			t.Errorf("kernel process %d was not reaped", i)
		}
		if _, err := os.Stat(runtimeDirs[i]); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("runtime directory %d still exists or stat failed unexpectedly: %v", i, err)
		}
	}
	if _, err := manager.Start(ctx, "python3"); err == nil {
		t.Fatal("Start() succeeded after Close()")
	}
}

func TestKernelManagerRejectsArbitraryLaunchCommands(t *testing.T) {
	python := findIPyKernelPython(t)
	_, err := NewKernelManager(KernelManagerConfig{
		RuntimeDir: t.TempDir(),
		Profiles: map[string]LaunchProfile{
			"python3": {Name: "python3", Command: python, Args: []string{"-c", "pass"}},
		},
	})
	if err == nil || !strings.Contains(err.Error(), connectionFilePlaceholder) {
		t.Fatalf("NewKernelManager() error = %v, want missing placeholder error", err)
	}

	manager := newTestKernelManager(t)
	defer manager.Close(context.Background())
	if _, err := manager.Start(context.Background(), "browser-supplied-command"); err == nil {
		t.Fatal("Start() accepted a non-allowlisted profile")
	}
}

func TestBoundedWriterKeepsOnlyTail(t *testing.T) {
	writer := newBoundedWriter(5)
	if _, err := writer.Write([]byte("1234")); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("5678")); err != nil {
		t.Fatal(err)
	}
	if got, want := writer.String(), "45678"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

func newTestKernelManager(t *testing.T) *KernelManager {
	t.Helper()
	python := findIPyKernelPython(t)
	profile := PythonLaunchProfile(python)
	manager, err := NewKernelManager(KernelManagerConfig{
		RuntimeDir:       filepath.Join(t.TempDir(), "runtime"),
		Profiles:         map[string]LaunchProfile{profile.Name: profile},
		StartupTimeout:   10 * time.Second,
		ReadinessTimeout: 5 * time.Second,
		GracefulTimeout:  2 * time.Second,
		TerminateTimeout: 2 * time.Second,
		KillTimeout:      2 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewKernelManager() error = %v", err)
	}
	return manager
}

func testKernelProcess(manager *KernelManager, id string) (*exec.Cmd, string) {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	kernel := manager.kernels[id]
	return kernel.command, kernel.runtimeDir
}

func assertOwnerOnlyPath(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("permissions for %s = %o, want %o", path, got, want)
	}
}

func waitForKernelState(t *testing.T, manager *KernelManager, id string, state KernelState) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		model, err := manager.Get(id)
		if err == nil && model.ExecutionState == string(state) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	model, err := manager.Get(id)
	t.Fatalf("kernel state = %+v, error = %v; want %s", model, err, state)
}

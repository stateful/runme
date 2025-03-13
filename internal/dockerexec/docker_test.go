//go:build docker_enabled

package dockerexec

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

const testAlpineImage = "alpine:3.19"

func TestDockerCommandContext(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()

	docker, err := New(
		&Options{
			Image: testAlpineImage,
		},
	)
	require.NoError(t, err)

	// Do not parallelize the warmup step.
	t.Run("Warmup", func(t *testing.T) {
		cmd := docker.CommandContext(context.Background(), "true")
		cmd.Dir = workingDir
		require.NoError(t, cmd.Start())
		require.NoError(t, cmd.Wait())
	})

	t.Run("NotTTY", func(t *testing.T) {
		t.Parallel()

		stdout := bytes.NewBuffer(nil)

		cmd := docker.CommandContext(context.Background(), "echo", "hello")
		cmd.Dir = workingDir
		cmd.Stdout = stdout

		require.NoError(t, cmd.Start())
		require.NoError(t, cmd.Wait())
		require.Equal(t, "hello\n", stdout.String())
	})

	t.Run("TTY", func(t *testing.T) {
		t.Parallel()

		stdout := bytes.NewBuffer(nil)

		cmd := docker.CommandContext(context.Background(), "echo", "hello")
		cmd.Dir = workingDir
		cmd.TTY = true
		cmd.Stdout = stdout

		require.NoError(t, cmd.Start())
		require.NoError(t, cmd.Wait())
		require.Equal(t, "hello\r\n", stdout.String())
	})

	t.Run("Shell", func(t *testing.T) {
		t.Parallel()

		stdout := bytes.NewBuffer(nil)

		cmd := docker.CommandContext(context.Background(), "sh", "-c", "echo hello")
		cmd.Dir = workingDir
		cmd.TTY = true
		cmd.Stdout = stdout

		require.NoError(t, cmd.Start())
		require.NoError(t, cmd.Wait())
		require.Equal(t, "hello\r\n", stdout.String())
	})
}

func TestDockerCommandContextWithCustomImage(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	buildContext := t.TempDir()

	err := os.WriteFile(
		filepath.Join(buildContext, "Dockerfile"),
		[]byte(`FROM `+testAlpineImage+`
COPY hello.txt /hello.txt
`),
		0o644,
	)
	require.NoError(t, err)
	err = os.WriteFile(
		filepath.Join(buildContext, "hello.txt"),
		[]byte("hello from file"),
		0o644,
	)
	require.NoError(t, err)

	docker, err := New(
		&Options{
			Image:        "runme-runner-test:latest",
			BuildContext: buildContext,
			Logger:       logger,
		},
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		testRemoveImage(t, ctx, docker)
	})

	workingDir := t.TempDir()

	t.Run("Basic", func(t *testing.T) {
		t.Parallel()

		stdout := bytes.NewBuffer(nil)

		cmd := docker.CommandContext(context.Background(), "echo", "hello")
		cmd.Dir = workingDir
		cmd.Stdout = stdout

		require.NoError(t, cmd.Start())
		require.NoError(t, cmd.Wait())
		require.Equal(t, "hello\n", stdout.String())
	})

	t.Run("BuildContextFile", func(t *testing.T) {
		t.Parallel()

		stdout := bytes.NewBuffer(nil)

		cmd := docker.CommandContext(context.Background(), "cat", "/hello.txt")
		cmd.Dir = workingDir
		cmd.Stdout = stdout

		require.NoError(t, cmd.Start())
		require.NoError(t, cmd.Wait())
		require.Equal(t, "hello from file", stdout.String())
	})
}

func testRemoveImage(t *testing.T, ctx context.Context, docker *Docker) {
	t.Helper()
	err := docker.RemoveImage(ctx)
	require.NoError(t, err)
}

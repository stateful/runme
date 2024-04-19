//go:build test_with_docker

package dockerexec

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDockerCommandContext(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()

	docker, err := New(
		&Options{
			Image: "alpine:3.19",
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

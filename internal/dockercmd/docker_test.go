//go:build test_with_docker

package dockercmd

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDockerKernel(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()

	factory, err := New(
		&Options{
			Image: "alpine:3.19",
		},
	)
	require.NoError(t, err)

	t.Run("Warmup", func(t *testing.T) {
		cmd := factory.CommandContext(context.Background(), "true")
		cmd.Dir = workingDir
		require.NoError(t, cmd.Start())
		require.NoError(t, cmd.Wait())
	})

	t.Run("NonTTY", func(t *testing.T) {
		t.Parallel()

		stdout := bytes.NewBuffer(nil)

		cmd := factory.CommandContext(context.Background(), "echo", "-n", "hello")
		cmd.Dir = workingDir
		cmd.Stdout = stdout

		require.NoError(t, cmd.Start())
		require.NoError(t, cmd.Wait())
		require.Equal(t, "hello", stdout.String())
	})

	t.Run("TTY", func(t *testing.T) {
		t.Parallel()

		stdout := bytes.NewBuffer(nil)

		cmd := factory.CommandContext(context.Background(), "echo", "hello")
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

		cmd := factory.CommandContext(context.Background(), "sh", "-c", "echo hello")
		cmd.Dir = workingDir
		cmd.TTY = true
		cmd.Stdout = stdout

		require.NoError(t, cmd.Start())
		require.NoError(t, cmd.Wait())
		require.Equal(t, "hello\r\n", stdout.String())
	})
}

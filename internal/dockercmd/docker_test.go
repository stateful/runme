//go:build test_with_docker

package dockercmd

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDockerKernel(t *testing.T) {
	workingDir := t.TempDir()

	factory, err := New(&Options{Image: "runme-kernel:latest"})
	require.NoError(t, err)

	t.Run("NonTTY", func(t *testing.T) {
		stdout := bytes.NewBuffer(nil)

		cmd := factory.CommandContext(context.Background(), "echo", "-n", "hello")
		cmd.Dir = workingDir
		cmd.Stdout = stdout

		require.NoError(t, cmd.Start())
		require.NoError(t, cmd.Wait())
		require.Equal(t, "hello", stdout.String())
	})

	t.Run("TTY", func(t *testing.T) {
		stdout := bytes.NewBuffer(nil)

		cmd := factory.CommandContext(context.Background(), "echo", "hello")
		cmd.Dir = workingDir
		cmd.TTY = true
		cmd.Stdout = stdout

		require.NoError(t, cmd.Start())
		require.NoError(t, cmd.Wait())
		require.Equal(t, "hello\r\n", stdout.String())
	})

	t.Run("shell", func(t *testing.T) {
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

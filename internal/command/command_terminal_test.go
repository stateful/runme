//go:build !windows

package command

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	runnerv2alpha1 "github.com/stateful/runme/v3/internal/gen/proto/go/runme/runner/v2alpha1"
)

func TestTerminalCommand_Options_Stdinwriter_Nil(t *testing.T) {
	cmd := NewTerminal(
		&Config{
			ProgramName: "bash",
			Mode:        runnerv2alpha1.CommandMode_COMMAND_MODE_TERMINAL,
		},
		Options{},
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	require.ErrorContains(t, cmd.Start(ctx), "stdin writer is nil")
}

func TestTerminalCommand(t *testing.T) {
	logger := zaptest.NewLogger(t)
	session := NewSession()

	stdinR, stdinW := io.Pipe()
	stdout := bytes.NewBuffer(nil)

	cmd := NewTerminal(
		&Config{
			ProgramName: "bash",
			Mode:        runnerv2alpha1.CommandMode_COMMAND_MODE_TERMINAL,
		},
		Options{
			Logger:      logger,
			Session:     session,
			StdinWriter: stdinW,
			Stdin:       stdinR,
			Stdout:      stdout,
		},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, cmd.Start(ctx))

	// TODO(adamb): on macOS is is not necessary, but on Linux
	// we need to wait for the shell to start before we start sending commands.
	time.Sleep(time.Second)

	_, err := stdinW.Write([]byte("export TEST_ENV=1\n"))
	require.NoError(t, err)
	_, err = stdinW.Write([]byte{0x04}) // EOT
	require.NoError(t, err)

	require.NoError(t, cmd.Wait())
	assert.Equal(t, []string{"TEST_ENV=1"}, session.GetEnv())
}

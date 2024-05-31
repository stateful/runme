//go:build !windows

package command

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	runnerv2alpha1 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/runner/v2alpha1"
)

func TestTerminalCommand_EnvPropagation(t *testing.T) {
	t.Parallel()

	session := NewSession()
	stdinR, stdinW := io.Pipe()
	stdout := bytes.NewBuffer(nil)

	factory := NewFactory(WithLogger(zaptest.NewLogger(t)))

	cmd := factory.Build(
		&ProgramConfig{
			ProgramName: "bash",
			Interactive: true,
			Mode:        runnerv2alpha1.CommandMode_COMMAND_MODE_TERMINAL,
		},
		CommandOptions{
			Session:     session,
			StdinWriter: stdinW,
			Stdin:       stdinR,
			Stdout:      stdout,
		},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, cmd.Start(ctx))

	// Terminal command sets up a trap on EXIT.
	// Wait for it before starting to send commands.
	expectContainLine(t, stdout, "trap -- \"__cleanup\" EXIT")

	_, err := stdinW.Write([]byte("export TEST_ENV=1\n"))
	require.NoError(t, err)
	// Wait for the prompt before sending the next command.
	expectContainLine(t, stdout, "$")
	_, err = stdinW.Write([]byte{0x04}) // EOT
	require.NoError(t, err)

	require.NoError(t, cmd.Wait())
	assert.Equal(t, []string{"TEST_ENV=1"}, session.GetAllEnv())
}

func expectContainLine(t *testing.T, r io.Reader, expected string) {
	t.Helper()

	for {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, expected) {
				return
			}
		}
		require.NoError(t, scanner.Err())
		time.Sleep(time.Millisecond * 100)
	}
}

func TestTerminalCommand_NonInteractive(t *testing.T) {
	t.Parallel()

	factory := NewFactory(WithLogger(zaptest.NewLogger(t)))

	stdinR, stdinW := io.Pipe()

	// Even if the [ProgramConfig] specifies that the command is non-interactive,
	// the factory should recognize it and change it to interactive.
	cmd := factory.Build(
		&ProgramConfig{
			ProgramName: "bash",
			Mode:        runnerv2alpha1.CommandMode_COMMAND_MODE_TERMINAL,
		},
		CommandOptions{
			StdinWriter: stdinW,
			Stdin:       stdinR,
		},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, cmd.Start(ctx))

	// TODO(adamb): on macOS is is not necessary, but on Linux
	// we need to wait for the shell to start before we start sending commands.
	time.Sleep(time.Second)
	_, err := stdinW.Write([]byte("echo -n test\n"))
	require.NoError(t, err)

	time.Sleep(time.Second)
	_, err = stdinW.Write([]byte{0x04}) // EOT
	require.NoError(t, err)
	require.NoError(t, cmd.Wait())
}

func TestTerminalCommand_OptionsStdinWriterNil(t *testing.T) {
	t.Parallel()

	factory := NewFactory(WithLogger(zaptest.NewLogger(t)))

	cmd := factory.Build(
		&ProgramConfig{
			ProgramName: "bash",
			Interactive: true,
			Mode:        runnerv2alpha1.CommandMode_COMMAND_MODE_TERMINAL,
		},
		CommandOptions{},
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	require.ErrorContains(t, cmd.Start(ctx), "stdin writer is nil")
}

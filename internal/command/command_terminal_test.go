//go:build !windows

package command

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/stateful/runme/v3/internal/sbuffer"
	"github.com/stateful/runme/v3/internal/session"
	"github.com/stateful/runme/v3/internal/testutils"
	runnerv2 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/runner/v2"
)

func TestTerminalCommand_EnvPropagation(t *testing.T) {
	t.Parallel()

	session, err := session.New()
	require.NoError(t, err)
	stdinR, stdinW := io.Pipe()
	stdout := sbuffer.New(nil)

	factory := NewFactory(WithLogger(zap.NewNop()))

	cmd, err := factory.Build(
		&ProgramConfig{
			ProgramName: "bash",
			Interactive: true,
			Mode:        runnerv2.CommandMode_COMMAND_MODE_TERMINAL,
		},
		CommandOptions{
			Session:     session,
			StdinWriter: stdinW,
			Stdin:       stdinR,
			Stdout:      stdout,
		},
	)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, cmd.Start(ctx))

	// Terminal command sets up a trap on EXIT.
	// Wait for it before starting to send commands.
	expectContainLines(ctx, t, stdout, []string{"trap -- \"__cleanup\" EXIT"})

	_, err = stdinW.Write([]byte("export TEST_ENV=1\n"))
	require.NoError(t, err)
	// Give the shell some time to process the command.
	time.Sleep(time.Second)

	// Wait for the prompt before sending the next command.
	prompt := "$"
	if testutils.IsRunningInDocker() {
		prompt = "#"
	}
	expectContainLines(ctx, t, stdout, []string{prompt})

	_, err = stdinW.Write([]byte("exit\n"))
	require.NoError(t, err)

	require.NoError(t, cmd.Wait(context.Background()))
	assert.Equal(t, []string{"TEST_ENV=1"}, session.GetAllEnv())
}

func TestTerminalCommand_Intro(t *testing.T) {
	session, err := session.New(session.WithSeedEnv(os.Environ()))
	require.NoError(t, err)

	stdinR, stdinW := io.Pipe()
	stdout := sbuffer.New(nil)

	factory := NewFactory(WithLogger(zaptest.NewLogger(t)))

	cmd, err := factory.Build(
		&ProgramConfig{
			ProgramName: "bash",
			Interactive: true,
			Mode:        runnerv2.CommandMode_COMMAND_MODE_TERMINAL,
		},
		CommandOptions{
			Session:     session,
			StdinWriter: stdinW,
			Stdin:       stdinR,
			Stdout:      stdout,
		},
	)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, cmd.Start(ctx))

	expectContainLines(ctx, t, stdout, []string{envSourceCmd, introSecondLine})
}

func expectContainLines(ctx context.Context, t *testing.T, r io.Reader, expected []string) {
	t.Helper()

	hits := make(map[string]bool, len(expected))
	buf := new(bytes.Buffer)
	output := new(strings.Builder)

	r = io.TeeReader(r, buf)

	for {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}

			_, _ = output.WriteString(line)

			for _, e := range expected {
				if strings.Contains(line, e) {
					hits[e] = true
				}
			}

			if len(hits) == len(expected) {
				return
			}
		}
		require.NoError(t, scanner.Err())

		select {
		case <-time.After(100 * time.Millisecond):
		case <-ctx.Done():
			t.Fatalf("error waiting for line %q, instead read %q: %s", expected, buf.String(), ctx.Err())
		}
	}
}

func TestTerminalCommand_NonInteractive(t *testing.T) {
	t.Parallel()

	factory := NewFactory(WithLogger(zaptest.NewLogger(t)))

	stdinR, stdinW := io.Pipe()

	// Even if the [ProgramConfig] specifies that the command is non-interactive,
	// the factory should recognize it and change it to interactive.
	cmd, err := factory.Build(
		&ProgramConfig{
			ProgramName: "bash",
			Mode:        runnerv2.CommandMode_COMMAND_MODE_TERMINAL,
		},
		CommandOptions{
			StdinWriter: stdinW,
			Stdin:       stdinR,
		},
	)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = cmd.Start(ctx)
	require.NoError(t, err)

	// TODO(adamb): on macOS is is not necessary, but on Linux
	// we need to wait for the shell to start before we start sending commands.
	time.Sleep(time.Second * 3)

	_, err = stdinW.Write([]byte("echo -n test\n"))
	require.NoError(t, err)

	// Wait for the command to finish and simulate more real-world usage.
	time.Sleep(time.Second)
	_, err = stdinW.Write([]byte{0x04}) // EOT
	require.NoError(t, err)

	err = cmd.Wait(context.Background())
	require.NoError(t, err)
}

func TestTerminalCommand_OptionsStdinWriterNil(t *testing.T) {
	t.Parallel()

	factory := NewFactory(WithLogger(zaptest.NewLogger(t)))

	cmd, err := factory.Build(
		&ProgramConfig{
			ProgramName: "bash",
			Interactive: true,
			Mode:        runnerv2.CommandMode_COMMAND_MODE_TERMINAL,
		},
		CommandOptions{},
	)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	require.ErrorContains(t, cmd.Start(ctx), "stdin writer is nil")
}

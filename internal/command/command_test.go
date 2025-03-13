package command

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/stateful/runme/v3/internal/session"
)

func init() {
	SetEnvDumpCommandForTesting()
}

func testExecuteCommand(
	t *testing.T,
	cfg *ProgramConfig,
	input io.Reader,
	expectedStdout string,
	expectedStderr string,
) {
	t.Helper()

	testExecuteCommandWithSession(t, cfg, nil, input, expectedStdout, expectedStderr)
}

func testExecuteCommandWithSession(
	t *testing.T,
	cfg *ProgramConfig,
	session *session.Session,
	input io.Reader,
	expectedStdout string,
	expectedStderr string,
) {
	t.Helper()

	if _, err := exec.LookPath(cfg.ProgramName); err != nil {
		t.Skipf("command %q not found", cfg.ProgramName)
	}

	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)

	factory := NewFactory(WithLogger(zaptest.NewLogger(t)))
	options := CommandOptions{
		Session: session,
		Stdout:  stdout,
		Stderr:  stderr,
		Stdin:   input,
	}
	command, err := factory.Build(cfg, options)
	require.NoError(t, err)
	err = command.Start(context.Background())
	require.NoError(t, err)
	err = command.Wait(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, expectedStdout, stdout.String())
	assert.Equal(t, expectedStderr, stderr.String())
}

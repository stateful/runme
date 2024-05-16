package command

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func testExecuteCommand(
	t *testing.T,
	cfg *ProgramConfig,
	input io.Reader,
	expectedStdout string,
	expectedStderr string,
) {
	t.Helper()

	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)

	options := Options{
		Stdout: stdout,
		Stderr: stderr,
		Stdin:  input,
	}

	command := NewFactory(nil, nil, zaptest.NewLogger(t)).Build(cfg, options)

	require.NoError(t, command.Start(context.Background()))
	assert.NoError(t, command.Wait())
	assert.Equal(t, expectedStdout, stdout.String())
	assert.Equal(t, expectedStderr, stderr.String())
}

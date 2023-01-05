//go:build !windows

package kernel

import (
	"bytes"
	"io"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func testCreateSession(t *testing.T) *Session {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()
	bashBin, err := exec.LookPath("bash")
	require.NoError(t, err)
	prompt, err := DetectPrompt(bashBin)
	require.NoError(t, err)
	sess, err := NewSession(
		prompt,
		"/usr/local/bin/bash",
		logger,
	)
	require.NoError(t, err)
	return sess
}

func TestSession(t *testing.T) {
	sess := testCreateSession(t)
	var buf bytes.Buffer
	exitCode, err := sess.Execute([]byte("echo TEST"), &buf)
	require.NoError(t, err)
	assert.Equal(t, 0, exitCode)
	assert.Equal(t, "echo TEST\r\nTEST\r\n", buf.String())
	require.NoError(t, sess.Destroy())
}

func TestSession_MultilineCommand(t *testing.T) {
	sess := testCreateSession(t)
	var buf bytes.Buffer
	exitCode, err := sess.Execute([]byte("sleep 1\necho TEST1\nsleep 1\necho TEST2"), &buf)
	require.NoError(t, err)
	assert.Equal(t, 0, exitCode)
	assert.Equal(t, "sleep 1\r\necho TEST1\r\nTEST1\r\nsleep 1\r\necho TEST2\r\nTEST2\r\n", buf.String())
	require.NoError(t, sess.Destroy())
}

func TestSession_Persistence(t *testing.T) {
	sess := testCreateSession(t)

	exitCode, err := sess.Execute([]byte("export TEST=test-value"), io.Discard)
	require.NoError(t, err)
	assert.Equal(t, 0, exitCode)

	var buf bytes.Buffer
	exitCode, err = sess.Execute([]byte("echo $TEST"), &buf)
	require.NoError(t, err)
	assert.Equal(t, 0, exitCode)
	assert.Equal(t, "echo $TEST\r\ntest-value\r\n", buf.String())

	require.NoError(t, sess.Destroy())
}

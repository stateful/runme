package kernel

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestSession(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()
	sess, err := NewSession(
		[]byte("bash-3.2$"),
		"/bin/bash",
		logger,
	)
	require.NoError(t, err)

	var buf bytes.Buffer
	exitCode, err := sess.Execute([]byte("echo TEST"), &buf)
	require.NoError(t, err)
	assert.Equal(t, 0, exitCode)
	assert.Equal(t, "echo TEST\r\nTEST", buf.String())
	require.NoError(t, sess.Destroy())
}

func TestSession_MultilineCommand(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()
	sess, err := NewSession(
		[]byte("bash-3.2$"),
		"/bin/bash",
		logger,
	)
	require.NoError(t, err)

	var buf bytes.Buffer
	exitCode, err := sess.Execute([]byte("sleep 1\necho TEST1\nsleep 1\necho TEST2"), &buf)
	require.NoError(t, err)
	assert.Equal(t, 0, exitCode)
	assert.Equal(t, "sleep 1\r\necho TEST1\r\nTEST1\r\nsleep 1\r\necho TEST2", buf.String())
	require.NoError(t, sess.Destroy())
}

func Test_bufferedWriter(t *testing.T) {
	data := []byte("prefix START==valid content==END suffix")
	for i := 16; i <= len(data); i *= 2 {
		t.Run("BufferSize"+strconv.Itoa(i), func(t *testing.T) {
			var buf bytes.Buffer
			w := newBufferedWriterSize(&buf, []byte("START"), []byte("END"), i)
			_, err := w.Write(data)
			require.NoError(t, err)
			require.NoError(t, w.Flush())
			assert.Equal(t, "==valid content==", buf.String())
		})
	}
}

func Test_bufferedWriter_SmallWrites(t *testing.T) {
	var buf bytes.Buffer
	w := newBufferedWriterSize(&buf, []byte("echo TEST\r\n"), []byte("\r\nbash-3.2$"), 4096)
	var err error
	_, err = w.Write([]byte("ec"))
	require.NoError(t, err)
	_, err = w.Write([]byte("ho"))
	require.NoError(t, err)
	_, err = w.Write([]byte(" TES"))
	require.NoError(t, err)
	_, err = w.Write([]byte("T\r\n"))
	require.NoError(t, err)
	_, err = w.Write([]byte("TEST\r\n"))
	require.NoError(t, err)
	_, err = w.Write([]byte("bash-3.2$ "))
	require.NoError(t, err)
	require.NoError(t, w.Flush())
	assert.Equal(t, "TEST", buf.String())
}

func Test_bufferedWriter_SmallWrites2(t *testing.T) {
	var buf bytes.Buffer
	w := newBufferedWriterSize(&buf, []byte("echo TEST\r\n"), []byte("\r\nbash-3.2$"), 4096)
	var err error
	_, err = w.Write([]byte("echo TEST\r\n"))
	require.NoError(t, err)
	_, err = w.Write([]byte("TEST\r\n"))
	require.NoError(t, err)
	_, err = w.Write([]byte("bash-3.2$ "))
	require.NoError(t, err)
	require.NoError(t, w.Flush())
	assert.Equal(t, "TEST", buf.String())
}

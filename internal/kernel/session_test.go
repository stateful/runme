//go:build !windows

package kernel

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func testCreateSession(t *testing.T, logger *zap.Logger) (*session, string) {
	if logger == nil {
		logger = zap.NewNop()
	}
	bashBin, err := exec.LookPath("bash")
	require.NoError(t, err)
	prompt, err := DetectPrompt(bashBin)
	require.NoError(t, err)
	sess, _, err := newSession(bashBin, string(prompt), logger)
	require.NoError(t, err)
	return sess, string(prompt)
}

func Test_session_Basic(t *testing.T) {
	sess, _ := testCreateSession(t, nil)

	data, exitCode, err := sess.Execute("echo Hello\n", time.Second)
	require.NoError(t, err)
	assert.Equal(t, "Hello", string(data))
	assert.Equal(t, 0, exitCode)

	err = sess.Close()
	require.NoError(t, err)
}

func Test_session_Multiline(t *testing.T) {
	sess, _ := testCreateSession(t, nil)

	// **Note** that this is a single command.
	// Multiple commands in a single string
	// are not allowed.
	data, exitCode, err := sess.Execute("echo 'Hello \\\nWorld'", time.Second)
	require.NoError(t, err)
	assert.Equal(t, "Hello \\\r\nWorld", string(data))
	assert.Equal(t, 0, exitCode)

	err = sess.Close()
	require.NoError(t, err)
}

func Test_session_Input(t *testing.T) {
	simulateUserInputLag := func() {
		<-time.After(time.Millisecond * 500)
	}

	sess, prompt := testCreateSession(t, nil)

	errC := make(chan error)
	var buf bytes.Buffer
	go func() {
		for {
			_, err := io.Copy(&buf, sess)
			if err != nil {
				errC <- err
				return
			}
		}
	}()

	simulateUserInputLag()
	err := sess.Send([]byte(`while read line
do
  echo "$line"
done
`))
	require.NoError(t, err)

	simulateUserInputLag()
	err = sess.Send([]byte("TEST\n"))
	require.NoError(t, err)

	simulateUserInputLag()
	err = sess.Send([]byte{'\u0003'}) // CTRL-C
	require.NoError(t, err)

	simulateUserInputLag()

	err = sess.Close()
	require.NoError(t, err)
	assert.Equal(t, "while read line\r\n> do\r\n>   echo \"$line\"\r\n> done\r\nTEST\r\nTEST\r\n^C\r\n"+prompt+" ", buf.String())
}

func Test_session_RawOutput(t *testing.T) {
	sess, _ := testCreateSession(t, nil)

	errC := make(chan error)
	var buf bytes.Buffer
	go func() {
		for {
			_, err := io.Copy(&buf, sess)
			if err != nil {
				errC <- err
				return
			}
		}
	}()

	err := sess.Send([]byte("echo 1\n"))
	require.NoError(t, err)
	err = sess.Send([]byte("sleep 1\n"))
	require.NoError(t, err)
	err = sess.Send([]byte("echo DONE\n"))
	require.NoError(t, err)

	<-time.After(time.Second * 2)

	err = sess.Close()
	require.NoError(t, err)
	assert.NotEmpty(t, buf.String())
}

func Test_session_Timeout(t *testing.T) {
	sess, _ := testCreateSession(t, nil)

	_, _, err := sess.Execute("sleep 2\n", time.Second)
	require.Error(t, err)

	err = sess.Close()
	require.NoError(t, err)
}

func Test_session_ExecuteWithWriter(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.SkipNow()
	}

	logger, _ := zap.NewDevelopment()
	defer logger.Sync()
	sess, _ := testCreateSession(t, logger)

	buf := bytes.NewBuffer(nil)

	// The command is structured that way so that it works in the CI
	// which has a limited pty width.
	exitCode, err := sess.ExecuteWithWriter(
		"echo 1 && sleep 1 && echo 2 && sleep 1 && echo 3\n",
		time.Second*3,
		buf,
	)
	require.NoError(t, err)
	assert.Equal(t, 0, exitCode)
	// TODO: the command should not be in the ouput.
	assert.Equal(t, "echo 1 && sleep 1 && echo 2 && sleep 1 && echo 3\r\n1\r\n2\r\n3\r\n", buf.String())

	err = sess.Close()
	require.NoError(t, err)
}

func Test_session_ChangePrompt(t *testing.T) {
	sess, _ := testCreateSession(t, nil)

	err := sess.ChangePrompt("RUNME")
	require.NoError(t, err)

	data, exitCode, err := sess.Execute("echo Hello\n", time.Second)
	require.NoError(t, err)
	assert.Equal(t, "Hello", string(data))
	assert.Equal(t, 0, exitCode)

	err = sess.Close()
	require.NoError(t, err)
}

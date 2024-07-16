//go:build !windows

package runner

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"
)

func init() {
	dumpCmd = "env -0"
}

func Test_command(t *testing.T) {
	t.Parallel()

	t.Run("Basic", func(t *testing.T) {
		t.Parallel()

		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)

		cmd, err := newCommand(
			context.Background(),
			&commandConfig{
				ProgramName: "bash",
				Stdout:      stdout,
				Stderr:      stderr,
				CommandMode: CommandModeInlineShell,
				Commands:    []string{"echo 1", "sleep 1", "echo 2"},
				Logger:      testCreateLogger(t),
			},
		)
		require.NoError(t, err)
		require.NoError(t, cmd.Start(context.Background()))
		require.NoError(t, cmd.Wait())
		data, err := io.ReadAll(stdout)
		assert.NoError(t, err)
		assert.Equal(t, "1\n2\n", string(data))
		data, err = io.ReadAll(stderr)
		assert.NoError(t, err)
		assert.Equal(t, "", string(data))
	})

	t.Run("BasicTempfile", func(t *testing.T) {
		t.Parallel()

		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)

		cmd, err := newCommand(
			context.Background(),
			&commandConfig{
				ProgramName: "bash",
				Stdout:      stdout,
				Stderr:      stderr,
				CommandMode: CommandModeTempFile,
				Commands:    []string{"echo 1", "sleep 1", "echo 2"},
				Logger:      testCreateLogger(t),
			},
		)
		require.NoError(t, err)
		require.NoError(t, cmd.Start(context.Background()))
		require.NoError(t, cmd.Wait())
		data, err := io.ReadAll(stdout)
		assert.NoError(t, err)
		assert.Equal(t, "1\n2\n", string(data))
		data, err = io.ReadAll(stderr)
		assert.NoError(t, err)
		assert.Equal(t, "", string(data))
	})

	t.Run("Shellscript", func(t *testing.T) {
		t.Parallel()

		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)

		cmd, err := newCommand(
			context.Background(),
			&commandConfig{
				ProgramName: "",
				LanguageID:  "shellscript",
				Stdout:      stdout,
				Stderr:      stderr,
				CommandMode: CommandModeTempFile,
				Script:      `echo "run this as shell script"`,
				Logger:      testCreateLogger(t),
			},
		)
		require.NoError(t, err)
		require.NoError(t, cmd.Start(context.Background()))
		require.NoError(t, cmd.Wait())
		data, err := io.ReadAll(stdout)
		assert.NoError(t, err)
		assert.Equal(t, "run this as shell script\n", string(data))
		data, err = io.ReadAll(stderr)
		assert.NoError(t, err)
		assert.Equal(t, "", string(data))
	})

	t.Run("JavaScript", func(t *testing.T) {
		t.Parallel()

		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)

		cmd, err := newCommand(
			context.Background(),
			&commandConfig{
				ProgramName: "",
				LanguageID:  "js",
				Stdout:      stdout,
				Stderr:      stderr,
				CommandMode: CommandModeTempFile,
				Script:      "console.log('1'); console.log('2')",
				Logger:      testCreateLogger(t),
			},
		)
		require.NoError(t, err)
		require.NoError(t, cmd.Start(context.Background()))
		require.NoError(t, cmd.Wait())
		data, err := io.ReadAll(stdout)
		assert.NoError(t, err)
		assert.Equal(t, "1\n2\n", string(data))
		data, err = io.ReadAll(stderr)
		assert.NoError(t, err)
		assert.Equal(t, "", string(data))
	})

	t.Run("JavaScriptEnv", func(t *testing.T) {
		t.Parallel()

		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)

		cmd, err := newCommand(
			context.Background(),
			&commandConfig{
				ProgramName: "/usr/bin/env node",
				LanguageID:  "js",
				Stdout:      stdout,
				Stderr:      stderr,
				CommandMode: CommandModeTempFile,
				Script:      "console.log('1'); console.log('2')",
				Logger:      testCreateLogger(t),
			},
		)
		require.NoError(t, err)
		require.NoError(t, cmd.Start(context.Background()))
		require.NoError(t, cmd.Wait())
		data, err := io.ReadAll(stdout)
		assert.NoError(t, err)
		assert.Equal(t, "1\n2\n", string(data))
		data, err = io.ReadAll(stderr)
		assert.NoError(t, err)
		assert.Equal(t, "", string(data))
	})

	t.Run("Nonexec resorts to cat", func(t *testing.T) {
		t.Parallel()

		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)

		cmd, err := newCommand(
			context.Background(),
			&commandConfig{
				ProgramName: "",
				LanguageID:  "sql",
				Stdout:      stdout,
				Stderr:      stderr,
				CommandMode: CommandModeTempFile,
				Script:      "SELECT * FROM users;",
				Logger:      testCreateLogger(t),
			},
		)
		require.NoError(t, err)
		require.NoError(t, cmd.Start(context.Background()))
		require.NoError(t, cmd.Wait())
		data, err := io.ReadAll(stdout)
		assert.NoError(t, err)
		assert.Contains(t, cmd.cmd.Path, "cat")
		assert.Equal(t, "SELECT * FROM users;", string(data))
		data, err = io.ReadAll(stderr)
		assert.NoError(t, err)
		assert.Equal(t, "", string(data))
	})

	t.Run("Tty", func(t *testing.T) {
		t.Parallel()

		stdin, stdinWriter := io.Pipe()
		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)

		cmd, err := newCommand(
			context.Background(),
			&commandConfig{
				ProgramName: "bash",
				Tty:         true,
				Stdin:       stdin,
				Stdout:      stdout,
				Stderr:      stderr,
				CommandMode: CommandModeInlineShell,
				Commands:    []string{"echo 1", "sleep 1", "echo 2"},
				Logger:      testCreateLogger(t),
			},
		)
		require.NoError(t, err)
		require.NoError(t, cmd.Start(context.Background()))
		go func() { _ = stdinWriter.Close() }()
		require.NoError(t, cmd.Wait())
		data, err := io.ReadAll(stdout)
		assert.NoError(t, err)
		assert.Equal(t, "1\r\n2\r\n", string(data))
		data, err = io.ReadAll(stderr)
		assert.NoError(t, err)
		assert.Equal(t, "", string(data))
	})

	t.Run("TtyNoScriptEOT", func(t *testing.T) {
		t.Parallel()

		stdin, stdinWriter := io.Pipe()
		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)

		cmd, err := newCommand(
			context.Background(),
			&commandConfig{
				ProgramName: "bash",
				Tty:         true,
				Stdin:       stdin,
				Stdout:      stdout,
				Stderr:      stderr,
				CommandMode: CommandModeInlineShell,
				Logger:      testCreateLogger(t),
			},
		)
		require.NoError(t, err)
		require.NoError(t, cmd.Start(context.Background()))

		go func() {
			time.Sleep(time.Second)
			_, _ = stdinWriter.Write([]byte{4})
			_ = stdinWriter.Close()
		}()

		require.NoError(t, cmd.Wait())
		data, err := io.ReadAll(stdout)
		assert.NoError(t, err)
		assert.Contains(t, string(data), "exit")
	})

	t.Run("Input", func(t *testing.T) {
		t.Parallel()

		stdin := new(bytes.Buffer)
		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)

		_, _ = stdin.WriteString("hello")

		cmd, err := newCommand(
			context.Background(),
			&commandConfig{
				ProgramName: "bash",
				Stdin:       stdin,
				Stdout:      stdout,
				Stderr:      stderr,
				CommandMode: CommandModeInlineShell,
				Script:      "cat - | tr a-z A-Z",
				Logger:      testCreateLogger(t),
			},
		)
		require.NoError(t, err)
		require.NoError(t, cmd.Start(context.Background()))
		require.NoError(t, cmd.Wait())
		data, err := io.ReadAll(stdout)
		assert.NoError(t, err)
		assert.Equal(t, "HELLO", string(data))
		data, err = io.ReadAll(stderr)
		assert.NoError(t, err)
		assert.Equal(t, "", string(data))
	})

	t.Run("InputTty", func(t *testing.T) {
		t.Parallel()

		stdin, stdinWriter := io.Pipe()
		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)

		cmd, err := newCommand(
			context.Background(),
			&commandConfig{
				ProgramName: "bash",
				Tty:         true,
				Stdin:       stdin,
				Stdout:      stdout,
				Stderr:      stderr,
				CommandMode: CommandModeInlineShell,
				Script:      "cat - | tr a-z A-Z",
				Logger:      testCreateLogger(t),
			},
		)
		require.NoError(t, err)
		require.NoError(t, cmd.Start(context.Background()))

		go func() {
			_, _ = stdinWriter.Write([]byte("hello\n"))
			time.Sleep(time.Second)
			_, _ = stdinWriter.Write([]byte("world\n"))
			time.Sleep(time.Second)
			_, _ = stdinWriter.Write([]byte{4})
			_ = stdinWriter.Close()
		}()

		assert.NoError(t, cmd.Wait())
		data, err := io.ReadAll(stdout)
		assert.NoError(t, err)
		assert.Contains(t, string(data), "hello\r\nHELLO\r\nworld\r\nWORLD\r\n")
		data, err = io.ReadAll(stderr)
		assert.NoError(t, err)
		assert.Equal(t, "", string(data))
	})

	t.Run("Stderr", func(t *testing.T) {
		t.Parallel()

		stdin := new(bytes.Buffer)
		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)

		_, _ = stdin.WriteString("hello")

		cmd, err := newCommand(
			context.Background(),
			&commandConfig{
				ProgramName: "bash",
				Stdin:       stdin,
				Stdout:      stdout,
				Stderr:      stderr,
				CommandMode: CommandModeInlineShell,
				Script:      "cat - 1>&2 2>/dev/null | tr a-z A-Z",
				Logger:      testCreateLogger(t),
			},
		)
		require.NoError(t, err)
		require.NoError(t, cmd.Start(context.Background()))
		require.NoError(t, cmd.Wait())
		// 1>&2 makes the FD 1 (aka stdout) refer to the same open FD
		// as FD 2 (aka stderr). The pipe is what originally the FD 1
		// pointed at. As nothing is written to the FD 1, the stdout is empty.
		data, err := io.ReadAll(stdout)
		assert.NoError(t, err)
		assert.Equal(t, "", string(data))
		// The input was not piped to "tr" due to streams redirection, hence,
		// it's available as-it-is in the stderr.
		data, err = io.ReadAll(stderr)
		assert.NoError(t, err)
		assert.Equal(t, "hello", string(data))
	})

	t.Run("InterruptTTY", func(t *testing.T) {
		t.Parallel()

		stdin, stdinWriter := io.Pipe()
		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)

		cmd, err := newCommand(
			context.Background(),
			&commandConfig{
				ProgramName: "bash",
				Tty:         true,
				Stdin:       stdin,
				Stdout:      stdout,
				Stderr:      stderr,
				CommandMode: CommandModeInlineShell,
				Logger:      testCreateLogger(t),
			},
		)
		require.NoError(t, err)
		require.NoError(t, cmd.Start(context.Background()))

		errc := make(chan error)
		go func() {
			defer close(errc)
			time.Sleep(time.Millisecond * 500)
			// This is only to simplify the output in newer bash versions.
			_, err := stdinWriter.Write([]byte("bind 'set enable-bracketed-paste off'\n"))
			errc <- err
			time.Sleep(time.Millisecond * 500)
			_, err = stdinWriter.Write([]byte("sleep 30\n"))
			errc <- err
			// cancel sleep
			time.Sleep(time.Millisecond * 500)
			_, err = stdinWriter.Write([]byte{3})
			errc <- err
			// terminate shell
			time.Sleep(time.Millisecond * 500)
			_, err = stdinWriter.Write([]byte{4})
			errc <- err
			errc <- stdinWriter.Close()
		}()
		for err := range errc {
			assert.NoError(t, err)
		}

		assert.ErrorContains(t, cmd.Wait(), "exit status 130")
	})

	t.Run("Env", func(t *testing.T) {
		t.Parallel()

		logger := testCreateLogger(t)
		session, err := NewSession(
			[]string{
				"TEST_OLD=value1",
				"TEST_OLD_CHANGED=value1",
				"TEST_OLD_UNSET=value1",
			},
			nil,
			logger,
		)
		require.NoError(t, err)

		cmd, err := newCommand(
			context.Background(),
			&commandConfig{
				ProgramName: "bash",
				Session:     session,
				Stdin:       bytes.NewBuffer(nil),
				Stdout:      io.Discard,
				Stderr:      io.Discard,
				CommandMode: CommandModeInlineShell,
				Commands: []string{
					"export TEST_NEW=value2",
					"export TEST_OLD_CHANGED=value2",
					"unset TEST_OLD_UNSET",
				},
				Logger: testCreateLogger(t),
			},
		)
		require.NoError(t, err)
		require.NoError(t, cmd.Start(context.Background()))
		require.NoError(t, cmd.Wait())

		sort := func(s []string) []string {
			slices.Sort(s)
			return s
		}

		env, err := cmd.Session.Envs()
		require.NoError(t, err)

		assert.EqualValues(
			t,
			sort([]string{
				"TEST_OLD=value1",
				"TEST_OLD_CHANGED=value2",
				"TEST_NEW=value2",
			}),
			sort(env),
		)
	})

	t.Run("Winsize", func(t *testing.T) {
		t.Parallel()

		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)
		stdin := new(bytes.Buffer)

		cmd, err := newCommand(
			context.Background(),
			&commandConfig{
				ProgramName: "bash",
				Stdout:      stdout,
				Stderr:      stderr,
				Stdin:       stdin,
				CommandMode: CommandModeInlineShell,
				Script:      "tput lines -T linux; tput cols -T linux",
				Logger:      testCreateLogger(t),
				Tty:         true,
				Winsize: &pty.Winsize{
					Cols: 100,
					Rows: 200,
				},
			},
		)
		require.NoError(t, err)
		require.NoError(t, cmd.Start(context.Background()))
		require.NoError(t, cmd.Wait())

		data, err := io.ReadAll(stdout)
		assert.NoError(t, err)
		assert.Equal(t, "200\r\n100\r\n", string(data))
	})
}

func Test_command_Stop(t *testing.T) {
	t.Parallel()

	cmd, err := newCommand(
		context.Background(),
		&commandConfig{
			ProgramName: "bash",
			Stdin:       bytes.NewBuffer(nil),
			Stdout:      io.Discard,
			Stderr:      io.Discard,
			CommandMode: CommandModeInlineShell,
			Commands:    []string{"sleep 30"},
			Logger:      testCreateLogger(t),
		},
	)
	require.NoError(t, err)
	require.NoError(t, cmd.Start(context.Background()))

	errc := make(chan error)
	go func() {
		time.Sleep(time.Second)
		errc <- cmd.Kill()
	}()
	assert.NoError(t, <-errc)

	var exiterr *exec.ExitError
	require.ErrorAs(t, cmd.Wait(), &exiterr)
	assert.Equal(t, os.Kill, exiterr.ProcessState.Sys().(syscall.WaitStatus).Signal())
}

func Test_exitCodeFromErr(t *testing.T) {
	cmd, err := newCommand(
		context.Background(),
		&commandConfig{
			ProgramName: "bash",
			Tty:         true,
			Stdin:       bytes.NewBuffer(nil),
			Stdout:      io.Discard,
			Stderr:      io.Discard,
			CommandMode: CommandModeInlineShell,
			Commands:    []string{"exit 99"},
			Logger:      testCreateLogger(t),
		},
	)
	require.NoError(t, err)
	require.NoError(t, cmd.Start(context.Background()))
	exiterr := cmd.Wait()
	assert.Error(t, exiterr)
	assert.Equal(t, 99, exitCodeFromErr(exiterr))
}

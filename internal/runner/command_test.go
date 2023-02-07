//go:build !windows

package runner

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"
)

func Test_command(t *testing.T) {
	t.Parallel()

	t.Run("Basic", func(t *testing.T) {
		t.Parallel()

		cmd, err := newCommand(
			&commandConfig{
				ProgramName: "bash",
				IsShell:     true,
				Commands:    []string{"echo 1", "sleep 1", "echo 2"},
			},
			testCreateLogger(t),
		)
		require.NoError(t, err)
		require.NoError(t, cmd.Start(context.Background()))
		require.NoError(t, cmd.Wait())
		data, err := io.ReadAll(cmd.Stdout)
		assert.NoError(t, err)
		assert.Equal(t, "1\n2\n", string(data))
		data, err = io.ReadAll(cmd.Stderr)
		assert.NoError(t, err)
		assert.Equal(t, "", string(data))
	})

	t.Run("Tty", func(t *testing.T) {
		t.Parallel()

		cmd, err := newCommand(
			&commandConfig{
				ProgramName: "bash",
				Tty:         true,
				IsShell:     true,
				Commands:    []string{"echo 1", "sleep 1", "echo 2"},
			},
			testCreateLogger(t),
		)
		require.NoError(t, err)
		require.NoError(t, cmd.Start(context.Background()))
		require.NoError(t, cmd.Wait())
		data, err := io.ReadAll(cmd.Stdout)
		assert.NoError(t, err)
		assert.Equal(t, "1\r\n2\r\n", string(data))
	})

	t.Run("InitialInput", func(t *testing.T) {
		t.Parallel()

		cmd, err := newCommand(
			&commandConfig{
				ProgramName: "bash",
				IsShell:     true,
				Script:      "cat - | tr a-z A-Z",
				Input:       []byte("Hello"),
			},
			testCreateLogger(t),
		)
		require.NoError(t, err)
		require.NoError(t, cmd.Start(context.Background()))
		require.NoError(t, cmd.Wait())
		data, err := io.ReadAll(cmd.Stdout)
		assert.NoError(t, err)
		assert.Equal(t, "HELLO", string(data))
	})

	t.Run("InputAsync", func(t *testing.T) {
		t.Parallel()

		cmd, err := newCommand(
			&commandConfig{
				ProgramName: "bash",
				Tty:         true,
				IsShell:     true,
				Script: `for ((i=0; i<3; i++)); do
  read -rp "input: " line
  echo "echo: $line"
done`,
			},
			testCreateLogger(t),
		)
		require.NoError(t, err)
		require.NoError(t, cmd.Start(context.Background()))

		go func() {
			// Simulate users input after each prompt
			// using sleep as a delay.
			time.Sleep(time.Second)
			_, _ = cmd.Stdin.Write([]byte("1\n"))
			time.Sleep(time.Second)
			_, _ = cmd.Stdin.Write([]byte("2\n"))
			time.Sleep(time.Second)
			_, _ = cmd.Stdin.Write([]byte("3\n"))
		}()

		require.NoError(t, cmd.Wait())
		data, err := io.ReadAll(cmd.Stdout)
		assert.NoError(t, err)
		assert.Equal(t, "input: 1\r\necho: 1\r\ninput: 2\r\necho: 2\r\ninput: 3\r\necho: 3\r\n", string(data))
	})

	t.Run("Stderr", func(t *testing.T) {
		t.Parallel()

		cmd, err := newCommand(
			&commandConfig{
				ProgramName: "bash",
				IsShell:     true,
				Script:      "cat - 1>&2 2>/dev/null | tr a-z A-Z",
				Input:       []byte("Hello"),
			},
			testCreateLogger(t),
		)
		require.NoError(t, err)
		require.NoError(t, cmd.Start(context.Background()))
		require.NoError(t, cmd.Wait())
		// 1>&2 makes the FD 1 (aka stdout) refer to the same open FD
		// as FD 2 (aka stderr). The pipe is what originally the FD 1
		// pointed at. As nothing is written to the FD 1, the stdout is empty.
		data, err := io.ReadAll(cmd.Stdout)
		assert.NoError(t, err)
		assert.Equal(t, "", string(data))
		// The input was not piped to "tr" due to streams redirection, hence,
		// it's available as-it-is in the stderr.
		data, err = io.ReadAll(cmd.Stderr)
		assert.NoError(t, err)
		assert.Equal(t, "Hello", string(data))
	})

	t.Run("TtyNoScriptEOT", func(t *testing.T) {
		t.Parallel()

		cmd, err := newCommand(
			&commandConfig{
				ProgramName: "bash",
				Tty:         true,
				IsShell:     true,
			},
			testCreateLogger(t),
		)
		require.NoError(t, err)
		require.NoError(t, cmd.Start(context.Background()))

		errc := make(chan error)
		go func() {
			time.Sleep(time.Second)
			_, err := cmd.Stdin.Write([]byte{4})
			errc <- err
		}()
		require.NoError(t, <-errc)

		require.NoError(t, cmd.Wait())
		data, err := io.ReadAll(cmd.Stdout)
		assert.NoError(t, err)
		assert.Contains(t, string(data), "exit")
	})

	t.Run("Interrupt", func(t *testing.T) {
		t.Parallel()

		cmd, err := newCommand(
			&commandConfig{
				ProgramName: "bash",
				Tty:         true,
				IsShell:     true,
				// This is only to simplify the output
				// in newer bash versions.
				Input: []byte("bind 'set enable-bracketed-paste off'\n"),
			},
			testCreateLogger(t),
		)
		require.NoError(t, err)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		require.NoError(t, cmd.Start(ctx))

		errc := make(chan error)
		go func() {
			defer close(errc)
			time.Sleep(time.Second)
			_, err := cmd.Stdin.Write([]byte("sleep 30\n"))
			errc <- err
			// cancel sleep
			time.Sleep(time.Millisecond * 500)
			_, err = cmd.Stdin.Write([]byte{3})
			errc <- err
			// terminate shell
			time.Sleep(time.Millisecond * 500)
			_, err = cmd.Stdin.Write([]byte{4})
			errc <- err
		}()
		for err := range errc {
			assert.NoError(t, err)
		}

		assert.ErrorContains(t, cmd.Wait(), "exit status 130")
		data, err := io.ReadAll(cmd.Stdout)
		assert.NoError(t, err)
		assert.Contains(t, string(data), "sleep 30")
		assert.Contains(t, string(data), "exit")
	})

	t.Run("Envs", func(t *testing.T) {
		t.Parallel()

		cmd, err := newCommand(
			&commandConfig{
				ProgramName: "bash",
				IsShell:     true,
				Envs: []string{
					"TEST_OLD=value1",
					"TEST_OLD_CHANGED=value1",
					"TEST_OLD_UNSET=value1",
				},
				Commands: []string{
					"export TEST_NEW=value2",
					"export TEST_OLD_CHANGED=value2",
					"unset TEST_OLD_UNSET",
				},
			},
			testCreateLogger(t),
		)
		require.NoError(t, err)
		require.NoError(t, cmd.Start(context.Background()))
		require.NoError(t, cmd.Wait())

		sort := func(s []string) []string {
			slices.Sort(s)
			return s
		}

		assert.EqualValues(
			t,
			sort([]string{
				"TEST_OLD=value1",
				"TEST_OLD_CHANGED=value2",
				"TEST_NEW=value2",
			}),
			sort(cmd.Envs),
		)
	})
}

func Test_exitCodeFromErr(t *testing.T) {
	cmd, err := newCommand(
		&commandConfig{
			ProgramName: "bash",
			Tty:         true,
			IsShell:     true,
			Commands:    []string{"exit 99"},
		},
		testCreateLogger(t),
	)
	require.NoError(t, err)
	require.NoError(t, cmd.Start(context.Background()))
	exiterr := cmd.Wait()
	assert.Error(t, exiterr)
	assert.Equal(t, 99, exitCodeFromErr(exiterr))
}

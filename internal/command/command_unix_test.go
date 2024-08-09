//go:build !windows

package command

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	runnerv2 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/runner/v2"
	"github.com/stateful/runme/v3/pkg/document"
	"github.com/stateful/runme/v3/pkg/document/identity"
)

func TestCommand(t *testing.T) {
	testCases := []struct {
		name           string
		cfg            *ProgramConfig
		env            []string
		input          []byte
		expectedStdout string
		expectedStderr string
	}{
		{
			name: "Echo",
			cfg: &ProgramConfig{
				ProgramName: "echo",
				Arguments:   []string{"-n", "test"},
				Interactive: true,
				Mode:        runnerv2.CommandMode_COMMAND_MODE_INLINE,
			},
			expectedStdout: "test",
		},
		{
			name: "EchoInteractive",
			cfg: &ProgramConfig{
				ProgramName: "echo",
				Arguments:   []string{"-n", "test"},
				Interactive: true,
				Mode:        runnerv2.CommandMode_COMMAND_MODE_INLINE,
			},
			expectedStdout: "test",
		},
		{
			name: "ShellScript",
			cfg: &ProgramConfig{
				ProgramName: "bash",
				Source: &runnerv2.ProgramConfig_Script{
					Script: "#!/usr/local/bin/bash\n\nset -x -e -o pipefail\n\necho -n test\n",
				},
				Mode: runnerv2.CommandMode_COMMAND_MODE_INLINE,
			},
			expectedStdout: "test",
			expectedStderr: "+ echo -n test\n+ __cleanup\n+ rv=0\n+ env -0\n+ exit 0\n",
		},
		{
			name: "Input",
			cfg: &ProgramConfig{
				ProgramName: "bash",
				Source: &runnerv2.ProgramConfig_Script{
					Script: "read line; echo $line | tr a-z A-Z",
				},
				Mode: runnerv2.CommandMode_COMMAND_MODE_INLINE,
			},
			input:          []byte("test\n"),
			expectedStdout: "TEST\n",
		},
		{
			name: "InputInteractive",
			cfg: &ProgramConfig{
				ProgramName: "bash",
				Source: &runnerv2.ProgramConfig_Script{
					Script: "read line; echo $line | tr a-z A-Z",
				},
				Interactive: true,
				Mode:        runnerv2.CommandMode_COMMAND_MODE_INLINE,
			},
			input:          []byte("test\n"),
			expectedStdout: "TEST\r\n",
		},
		{
			name: "StdoutStderr",
			cfg: &runnerv2.ProgramConfig{
				ProgramName: "bash",
				Source: &runnerv2.ProgramConfig_Commands{
					Commands: &runnerv2.ProgramConfig_CommandList{
						Items: []string{
							"echo test | tee >(cat >&2)",
						},
					},
				},
				Mode: runnerv2.CommandMode_COMMAND_MODE_INLINE,
			},
			expectedStdout: "test\n",
			expectedStderr: "test\n",
		},
		{
			// This test demonstrates a behaviour for some languages
			// like TypeScript running with "deno run". In the ideal world,
			// the caller should handle that and pass "deno" as ProgramName and
			// "run" in Arguments.
			name: "MultiWordProgramName",
			cfg: &runnerv2.ProgramConfig{
				ProgramName: "echo -n test",
				Mode:        runnerv2.CommandMode_COMMAND_MODE_INLINE,
			},
			expectedStdout: "test",
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			testExecuteCommand(
				t,
				tc.cfg,
				bytes.NewReader(tc.input),
				tc.expectedStdout,
				tc.expectedStderr,
			)
		})
	}
}

func TestCommand_FromCodeBlocks(t *testing.T) {
	testCases := []struct {
		name           string
		source         string
		env            []string
		input          []byte
		expectedStdout string
		expectedStderr string
	}{
		{
			name:           "BasicShell",
			source:         "```bash\necho -n test\n```",
			expectedStdout: "test",
		},
		{
			name:           "ShellScript",
			source:         "```shellscript\n#!/usr/local/bin/bash\n\nset -x -e -o pipefail\n\necho -n test\n```",
			expectedStdout: "test",
			expectedStderr: "+ echo -n test\n", // due to -x
		},
		{
			name:           "ShellScriptInteractive",
			source:         "```shellscript {\"interactive\": true}\n#!/usr/local/bin/bash\n\nset -x -e -o pipefail\n\necho -n test\n```",
			expectedStdout: "+ echo -n test\r\ntest", // due to -x
		},
		{
			name:           "Python",
			source:         "```py\nprint('test')\n```",
			expectedStdout: "test\n",
		},
		{
			name:           "PythonInteractive",
			source:         "```py {\"interactive\": true}\nprint('test')\n```",
			expectedStdout: "test\r\n",
		},
		{
			name:           "JavaScript",
			source:         "```js\nconsole.log('1'); console.log('2')\n```",
			expectedStdout: "1\n2\n",
		},
		{
			name:   "Empty",
			source: "```sh\n```",
		},
		{
			name:           "WithInput",
			source:         "```bash\nread line; echo $line | tr a-z A-Z\n```",
			input:          []byte("test\n"),
			expectedStdout: "TEST\n",
		},
		{
			name:           "WithInputInteractive",
			source:         "```bash {\"interactive\": true}\nread line; echo $line | tr a-z A-Z\n```",
			input:          []byte("test\n"),
			expectedStdout: "TEST\r\n",
		},
		{
			name:           "Env",
			source:         "```bash\necho -n $MY_ENV\n```",
			env:            []string{"MY_ENV=hello"},
			expectedStdout: "hello",
		},
		{
			name:           "Interpreter",
			source:         "```sh { \"interpreter\": \"bash\" }\necho -n test\n```",
			expectedStdout: "test",
		},
		{
			name:           "FrontmatterShell",
			source:         "---\nshell: bash\n---\n```sh\necho -n $0 | xargs basename\n```",
			expectedStdout: "bash\n",
		},
		{
			name:           "DefaultToCat",
			source:         "```\nSELECT * FROM users;\n```",
			expectedStdout: "SELECT * FROM users;",
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			idResolver := identity.NewResolver(identity.AllLifecycleIdentity)

			doc := document.New([]byte(tc.source), idResolver)
			node, err := doc.Root()
			require.NoError(t, err)

			blocks := document.CollectCodeBlocks(node)
			require.Len(t, blocks, 1)

			cfg, err := NewProgramConfigFromCodeBlock(blocks[0])
			require.NoError(t, err)

			cfg.Env = tc.env

			testExecuteCommand(
				t,
				cfg,
				bytes.NewReader(tc.input),
				tc.expectedStdout,
				tc.expectedStderr,
			)
		})
	}
}

func TestCommand_Getters(t *testing.T) {
	t.Parallel()

	factory := NewFactory(WithLogger(zaptest.NewLogger(t)))

	cfg := &ProgramConfig{
		ProgramName: "sleep",
		Arguments:   []string{"1"},
		Mode:        runnerv2.CommandMode_COMMAND_MODE_INLINE,
	}

	cmd, err := factory.Build(cfg, CommandOptions{})
	require.NoError(t, err)
	require.NoError(t, cmd.Start(context.Background()))
	require.True(t, cmd.Running())
	require.Greater(t, cmd.Pid(), 1)
	require.NoError(t, cmd.Wait())
}

func TestCommand_InvalidProgram(t *testing.T) {
	t.Parallel()

	factory := NewFactory(WithLogger(zaptest.NewLogger(t)))

	cfg := &ProgramConfig{
		ProgramName: "invalidProgram",
		Source: &runnerv2.ProgramConfig_Commands{
			Commands: &runnerv2.ProgramConfig_CommandList{
				Items: []string{"echo -n test"},
			},
		},
		Mode: runnerv2.CommandMode_COMMAND_MODE_INLINE,
	}

	cmd, err := factory.Build(cfg, CommandOptions{})
	require.NoError(t, err)
	err = cmd.Start(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed program lookup \"invalidProgram\"")
}

func TestCommand_InvalidScript(t *testing.T) {
	t.Parallel()

	factory := NewFactory(WithLogger(zaptest.NewLogger(t)))

	cfg := &ProgramConfig{
		ProgramName: "bash",
		Source: &runnerv2.ProgramConfig_Commands{
			Commands: &runnerv2.ProgramConfig_CommandList{
				Items: []string{
					"failhereplease",
					"echo executed",
				},
			},
		},
		Mode: runnerv2.CommandMode_COMMAND_MODE_INLINE,
	}

	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)

	cmd, err := factory.Build(cfg, CommandOptions{Stdout: stdout, Stderr: stderr})
	require.NoError(t, err)

	err = cmd.Start(context.Background())
	require.NoError(t, err)
	err = cmd.Wait()
	require.Error(t, err)
	require.Equal(t, "", stdout.String())
	require.Contains(t, stderr.String(), "failhereplease: command not found")
}

// TestCommand_SetWinsize validates if it's possible to set the window size for a command.
// Overall, for any interactive command it should be possible.
// Check out command_terminal_test.go for more details.
func TestCommand_SetWinsize(t *testing.T) {
	t.Parallel()

	factory := NewFactory(WithLogger(zaptest.NewLogger(t)))

	t.Run("InlineInteractive", func(t *testing.T) {
		t.Parallel()

		stdout := bytes.NewBuffer(nil)

		cmd, err := factory.Build(
			&ProgramConfig{
				ProgramName: "bash",
				Source: &runnerv2.ProgramConfig_Commands{
					Commands: &runnerv2.ProgramConfig_CommandList{
						Items: []string{"sleep 1", "tput cols -T linux", "tput lines -T linux"},
					},
				},
				Interactive: true,
				Mode:        runnerv2.CommandMode_COMMAND_MODE_INLINE,
			},
			CommandOptions{Stdout: stdout},
		)
		require.NoError(t, err)

		err = cmd.Start(context.Background())
		require.NoError(t, err)
		err = SetWinsize(cmd, &Winsize{Rows: 45, Cols: 56, X: 0, Y: 0})
		require.NoError(t, err)
		err = cmd.Wait()
		require.NoError(t, err)
		require.Equal(t, "56\r\n45\r\n", stdout.String())
	})

	t.Run("Terminal", func(t *testing.T) {
		t.Parallel()

		stdout := bytes.NewBuffer(nil)
		stdinR, stdinW := io.Pipe()

		// Even if the [ProgramConfig] specifies that the command is non-interactive,
		// the factory should recognize it and change it to interactive.
		cmd, err := factory.Build(
			&ProgramConfig{
				ProgramName: "bash",
				Mode:        runnerv2.CommandMode_COMMAND_MODE_TERMINAL,
				Interactive: true,
				Env:         []string{"TERM=xterm"},
			},
			CommandOptions{
				StdinWriter: stdinW,
				Stdin:       stdinR,
				Stdout:      stdout,
			},
		)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = cmd.Start(ctx)
		require.NoError(t, err)

		// TODO(adamb): on macOS is is not necessary, but on Linux
		// we need to wait for the shell to start before we start sending commands.
		time.Sleep(time.Second)

		err = SetWinsize(cmd, &Winsize{Rows: 45, Cols: 56, X: 0, Y: 0})
		require.NoError(t, err)
		_, err = stdinW.Write([]byte("tput cols -T linux; tput lines -T linux\n"))
		require.NoError(t, err)
		_, err = stdinW.Write([]byte{0x04}) // EOT
		require.NoError(t, err)
		err = cmd.Wait()
		require.NoError(t, err, "command failed due to: %s", stdout.String())
		require.Contains(t, stdout.String(), "56\r\n45\r\n")
	})
}

func TestCommand_Session(t *testing.T) {
	setterCfg := &ProgramConfig{
		ProgramName: "bash",
		Source: &runnerv2.ProgramConfig_Commands{
			Commands: &runnerv2.ProgramConfig_CommandList{
				Items: []string{"export TEST_ENV=test1"},
			},
		},
		Mode: runnerv2.CommandMode_COMMAND_MODE_INLINE,
	}
	getterCfg := &ProgramConfig{
		ProgramName: "bash",
		Source: &runnerv2.ProgramConfig_Commands{
			Commands: &runnerv2.ProgramConfig_CommandList{
				Items: []string{"echo -n \"TEST_ENV equals $TEST_ENV\""},
			},
		},
		Mode: runnerv2.CommandMode_COMMAND_MODE_INLINE,
	}

	sess := NewSession()

	factory := NewFactory(WithLogger(zaptest.NewLogger(t)))

	setterCmd, err := factory.Build(setterCfg, CommandOptions{Session: sess})
	require.NoError(t, err)
	require.NoError(t, setterCmd.Start(context.Background()))
	require.NoError(t, setterCmd.Wait())
	require.Equal(t, []string{"TEST_ENV=test1"}, sess.GetAllEnv())

	stdout := bytes.NewBuffer(nil)
	getterCmd, err := factory.Build(getterCfg, CommandOptions{Session: sess, Stdout: stdout})
	require.NoError(t, err)
	require.NoError(t, getterCmd.Start(context.Background()))
	require.NoError(t, getterCmd.Wait())
	require.Equal(t, "TEST_ENV equals test1", stdout.String())
}

func TestCommand_SimulateCtrlC(t *testing.T) {
	idResolver := identity.NewResolver(identity.AllLifecycleIdentity)
	doc := document.New([]byte("```sh {\"interactive\": true}\nbash\n```"), idResolver)
	node, err := doc.Root()
	require.NoError(t, err)
	blocks := document.CollectCodeBlocks(node)
	require.Len(t, blocks, 1)

	cfg, err := NewProgramConfigFromCodeBlock(blocks[0])
	require.NoError(t, err)

	stdinR, stdinW := io.Pipe()
	stdout := bytes.NewBuffer(nil)

	factory := NewFactory(WithLogger(zaptest.NewLogger(t)))
	cmd, err := factory.Build(cfg, CommandOptions{Stdin: stdinR, Stdout: stdout})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, cmd.Start(ctx))

	errc := make(chan error)
	go func() {
		defer close(errc)

		time.Sleep(time.Millisecond * 500)
		_, err = stdinW.Write([]byte("sleep 30\n"))
		errc <- err

		// cancel sleep
		time.Sleep(time.Millisecond * 500)
		_, err = stdinW.Write([]byte{3})
		errc <- err

		// terminate shell
		time.Sleep(time.Millisecond * 500)
		_, err = stdinW.Write([]byte{4})
		errc <- err

		// close writer; it's not needed
		errc <- stdinW.Close()
	}()
	for err := range errc {
		require.NoError(t, err)
	}

	require.EqualError(t, cmd.Wait(), "exit status 130")
}

func TestCommand_StopWithSignal(t *testing.T) {
	t.Parallel()

	factory := NewFactory(WithLogger(zaptest.NewLogger(t)))

	cfg := &ProgramConfig{
		ProgramName: "sleep",
		Arguments:   []string{"10"},
		Mode:        runnerv2.CommandMode_COMMAND_MODE_INLINE,
	}

	t.Run("SIGINT", func(t *testing.T) {
		cmd, err := factory.Build(cfg, CommandOptions{})
		require.NoError(t, err)
		require.NoError(t, cmd.Start(context.Background()))

		errc := make(chan error, 1)
		go func() {
			errc <- cmd.Signal(os.Interrupt)
		}()

		require.EqualError(t, cmd.Wait(), "signal: interrupt")
		require.NoError(t, <-errc)
	})

	t.Run("SIGKILL", func(t *testing.T) {
		cmd, err := factory.Build(cfg, CommandOptions{})
		require.NoError(t, err)
		require.NoError(t, cmd.Start(context.Background()))

		errc := make(chan error, 1)
		go func() {
			errc <- cmd.Signal(os.Kill)
		}()

		require.EqualError(t, cmd.Wait(), "signal: killed")
		require.NoError(t, <-errc)
	})
}

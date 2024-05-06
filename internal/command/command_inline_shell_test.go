package command

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/stateful/runme/v3/internal/document"
	"github.com/stateful/runme/v3/internal/document/identity"
	runnerv2alpha1 "github.com/stateful/runme/v3/internal/gen/proto/go/runme/runner/v2alpha1"
)

func TestInlineShellCommand(t *testing.T) {
	t.Parallel()

	t.Run("Echo", func(t *testing.T) {
		t.Parallel()

		cfg := &ProgramConfig{
			ProgramName: "bash",
			Source: &runnerv2alpha1.ProgramConfig_Commands{
				Commands: &runnerv2alpha1.ProgramConfig_CommandList{
					Items: []string{"echo -n test"},
				},
			},
			Mode: runnerv2alpha1.CommandMode_COMMAND_MODE_INLINE,
		}

		testExecuteCommand(t, cfg, nil, "test", "")
	})

	t.Run("ShellScript", func(t *testing.T) {
		t.Parallel()

		cfg := &ProgramConfig{
			ProgramName: "bash",
			Source: &runnerv2alpha1.ProgramConfig_Script{
				Script: "#!/usr/local/bin/bash\n\nset -x -e -o pipefail\n\necho -n test\n",
			},
			Mode: runnerv2alpha1.CommandMode_COMMAND_MODE_INLINE,
		}

		testExecuteCommand(t, cfg, nil, "test", "+ echo -n test\n+ __cleanup\n+ rv=0\n+ env -0\n+ exit 0\n")
	})

	t.Run("Input", func(t *testing.T) {
		t.Parallel()

		cfg := &ProgramConfig{
			ProgramName: "bash",
			Source: &runnerv2alpha1.ProgramConfig_Script{
				Script: "read line; echo $line | tr a-z A-Z",
			},
			Mode: runnerv2alpha1.CommandMode_COMMAND_MODE_INLINE,
		}

		testExecuteCommand(t, cfg, bytes.NewReader([]byte("test\n")), "TEST\n", "")
	})

	t.Run("InputInteractive", func(t *testing.T) {
		t.Parallel()

		cfg := &ProgramConfig{
			ProgramName: "bash",
			Source: &runnerv2alpha1.ProgramConfig_Script{
				Script: "read line; echo $line | tr a-z A-Z",
			},
			Interactive: true,
			Mode:        runnerv2alpha1.CommandMode_COMMAND_MODE_INLINE,
		}

		testExecuteCommand(t, cfg, bytes.NewReader([]byte("test\n")), "TEST\r\n", "")
	})

	t.Run("SetWinsize", func(t *testing.T) {
		stdout := bytes.NewBuffer(nil)

		cmd := newInlineShell(
			newVirtual(
				&ProgramConfig{
					ProgramName: "bash",
					Source: &runnerv2alpha1.ProgramConfig_Commands{
						Commands: &runnerv2alpha1.ProgramConfig_CommandList{
							Items: []string{"sleep 1", "tput cols -T linux", "tput lines -T linux"},
						},
					},
					Interactive: true,
					Mode:        runnerv2alpha1.CommandMode_COMMAND_MODE_INLINE,
				},
				Options{Stdout: stdout},
			),
		)
		require.NoError(t, cmd.Start(context.Background()))
		require.NoError(t, SetWinsize(cmd, &Winsize{Rows: 24, Cols: 80, X: 0, Y: 0}))
		require.NoError(t, cmd.Wait())
		require.Equal(t, "80\r\n24\r\n", stdout.String())
	})
}

func TestInlineShellCommandFromCodeBlocksWithInputUsingPipe(t *testing.T) {
	idResolver := identity.NewResolver(identity.AllLifecycleIdentity)
	logger := zaptest.NewLogger(t)

	t.Run("Cat", func(t *testing.T) {
		source := "```sh\ncat - | tr a-z A-Z\n```"
		doc := document.New([]byte(source), idResolver)
		node, err := doc.Root()
		require.NoError(t, err)

		blocks := document.CollectCodeBlocks(node)
		require.Len(t, blocks, 1)

		cfg, err := NewProgramConfigFromCodeBlock(blocks[0])
		require.NoError(t, err)
		cfg.Interactive = true

		stdinR, stdinW := io.Pipe()
		stdout := bytes.NewBuffer(nil)

		command := NewFactory(nil, nil).Build(cfg, Options{Stdin: stdinR, Stdout: stdout, Logger: logger})

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		require.NoError(t, command.Start(ctx))

		_, err = stdinW.Write([]byte("unit tests\n"))
		require.NoError(t, err)
		_, err = stdinW.Write([]byte{0x04}) // EOT
		require.NoError(t, err)

		require.NoError(t, command.Wait())
		require.Equal(t, "UNIT TESTS\r\n", stdout.String())
	})

	t.Run("Read", func(t *testing.T) {
		source := "```sh\nread name\necho \"My name is $name\"\n```"
		doc := document.New([]byte(source), idResolver)
		node, err := doc.Root()
		require.NoError(t, err)

		blocks := document.CollectCodeBlocks(node)
		require.Len(t, blocks, 1)

		cfg, err := NewProgramConfigFromCodeBlock(blocks[0])
		require.NoError(t, err)
		cfg.Interactive = true

		stdinR, stdinW := io.Pipe()
		stdout := bytes.NewBuffer(nil)

		command := NewFactory(nil, nil).Build(cfg, Options{Stdin: stdinR, Stdout: stdout, Logger: logger})

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		require.NoError(t, command.Start(ctx))

		_, err = stdinW.Write([]byte("Unit Test\n"))
		require.NoError(t, err)

		require.NoError(t, command.Wait())
		require.Equal(t, "My name is Unit Test\r\n", stdout.String())
	})

	t.Run("SimulateCtrlC", func(t *testing.T) {
		// Using sh start bash. We need to go deeper...
		source := "```sh\nbash\n```"

		doc := document.New([]byte(source), idResolver)
		node, err := doc.Root()
		require.NoError(t, err)

		blocks := document.CollectCodeBlocks(node)
		require.Len(t, blocks, 1)

		cfg, err := NewProgramConfigFromCodeBlock(blocks[0])
		require.NoError(t, err)
		cfg.Interactive = true

		stdinR, stdinW := io.Pipe()
		stdout := bytes.NewBuffer(nil)

		command := NewFactory(nil, nil).Build(cfg, Options{Stdin: stdinR, Stdout: stdout, Logger: logger})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		require.NoError(t, command.Start(ctx))

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

		require.EqualError(t, command.Wait(), "exit status 130")
	})
}

func TestInlineShellCommandWithSession(t *testing.T) {
	setterCfg := &ProgramConfig{
		ProgramName: "bash",
		Source: &runnerv2alpha1.ProgramConfig_Commands{
			Commands: &runnerv2alpha1.ProgramConfig_CommandList{
				Items: []string{"export TEST_ENV=test1"},
			},
		},
		Mode: runnerv2alpha1.CommandMode_COMMAND_MODE_INLINE,
	}
	getterCfg := &ProgramConfig{
		ProgramName: "bash",
		Source: &runnerv2alpha1.ProgramConfig_Commands{
			Commands: &runnerv2alpha1.ProgramConfig_CommandList{
				Items: []string{"echo -n \"TEST_ENV equals $TEST_ENV\""},
			},
		},
		Mode: runnerv2alpha1.CommandMode_COMMAND_MODE_INLINE,
	}

	sess := NewSession()

	commandSetter := newInlineShell(
		newNative(setterCfg, Options{Session: sess}),
	)
	require.NoError(t, commandSetter.Start(context.Background()))
	require.NoError(t, commandSetter.Wait())
	require.Equal(t, []string{"TEST_ENV=test1"}, sess.GetEnv())

	stdout := bytes.NewBuffer(nil)
	commandGetter := newInlineShell(
		newNative(getterCfg, Options{Session: sess, Stdout: stdout}),
	)
	require.NoError(t, commandGetter.Start(context.Background()))
	require.NoError(t, commandGetter.Wait())
	require.Equal(t, "TEST_ENV equals test1", stdout.String())
}

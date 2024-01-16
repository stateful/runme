//go:build !windows

package command

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/stateful/runme/internal/document"
	"github.com/stateful/runme/internal/document/identity"
	runnerv2alpha1 "github.com/stateful/runme/internal/gen/proto/go/runme/runner/v2alpha1"
)

var (
	testConfigBasicProgram = &Config{
		ProgramName: "echo",
		Arguments:   []string{"-n", "test"},
		Mode:        runnerv2alpha1.CommandMode_COMMAND_MODE_INLINE,
	}

	testConfigShellProgram = &Config{
		ProgramName: "bash",
		Source: &runnerv2alpha1.ProgramConfig_Commands{
			Commands: &runnerv2alpha1.ProgramConfig_CommandList{
				Items: []string{"echo -n test"},
			},
		},
		Mode: runnerv2alpha1.CommandMode_COMMAND_MODE_INLINE,
	}
)

func init() {
	envDumpCommand = "env -0"
}

func TestExecutionCommandFromCodeBlocks(t *testing.T) {
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)
	defer logger.Sync()

	testCases := []struct {
		name                  string
		source                string
		env                   []string
		input                 []byte
		nativeExpectedStdout  string
		nativeExpectedStderr  string
		virtualExpectedStdout string
	}{
		{
			name:                 "BasicShell",
			source:               "```bash\necho -n test\n```",
			nativeExpectedStdout: "test",
		},
		{
			name:                  "ShellScript",
			source:                "```shellscript\n#!/usr/local/bin/bash\n\nset -x -e -o pipefail\n\necho -n test\n```",
			nativeExpectedStdout:  "test",
			nativeExpectedStderr:  "+ echo -n test\n", // due to -x
			virtualExpectedStdout: "+ echo -n test\r\ntest",
		},
		{
			name:                  "Python",
			source:                "```python\nprint('test')\n```",
			nativeExpectedStdout:  "test\n",
			virtualExpectedStdout: "test\r\n",
		},
		{
			name:                  "JavaScript",
			source:                "```js\nconsole.log('1'); console.log('2')\n```",
			nativeExpectedStdout:  "1\n2\n",
			virtualExpectedStdout: "1\r\n2\r\n",
		},
		{
			name:   "Empty",
			source: "```sh\n```",
		},
		{
			name:                  "WithInput",
			source:                "```bash\nread line; echo $line | tr a-z A-Z\n```",
			input:                 []byte("test\n"),
			nativeExpectedStdout:  "TEST\n",
			virtualExpectedStdout: "TEST\r\n",
		},
		{
			name:                 "Env",
			source:               "```bash\necho -n $MY_ENV\n```",
			env:                  []string{"MY_ENV=hello"},
			nativeExpectedStdout: "hello",
		},
		{
			name:                 "Interpreter",
			source:               "```sh { \"interpreter\": \"bash\" }\necho -n test\n```",
			nativeExpectedStdout: "test",
		},
		{
			name:                 "FrontmatterShell",
			source:               "---\nshell: bash\n---\n```sh\necho -n test\n```",
			nativeExpectedStdout: "test",
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run("NativeCommand", func(t *testing.T) {
			t.Parallel()

			tc := tc

			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				testExecuteNativeCommand(
					t,
					[]byte(tc.source),
					tc.env,
					bytes.NewReader(tc.input),
					tc.nativeExpectedStdout,
					tc.nativeExpectedStderr,
					logger.Named(t.Name()),
				)
			})
		})

		t.Run("VirtualCommand", func(t *testing.T) {
			t.Parallel()

			tc := tc

			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				expectedOutput := tc.nativeExpectedStdout
				if tc.virtualExpectedStdout != "" {
					expectedOutput = tc.virtualExpectedStdout
				}

				testExecuteVirtualCommand(
					t,
					[]byte(tc.source),
					tc.env,
					bytes.NewReader(tc.input),
					expectedOutput,
					logger.Named(t.Name()),
				)
			})
		})
	}
}

func testExecuteNativeCommand(
	t *testing.T,
	source []byte,
	env []string,
	input io.Reader,
	expectedStdout string,
	expectedStderr string,
	logger *zap.Logger,
) {
	t.Helper()

	idResolver := identity.NewResolver(identity.AllLifecycleIdentity)

	doc := document.New(source, idResolver)
	node, err := doc.Root()
	require.NoError(t, err)

	blocks := document.CollectCodeBlocks(node)
	require.Len(t, blocks, 1)

	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)

	cfg, err := NewConfigFromCodeBlock(blocks[0])
	require.NoError(t, err)

	options := &NativeCommandOptions{
		Session: MustNewSessionWithEnv(env...),
		Stdout:  stdout,
		Stderr:  stderr,
		Stdin:   input,
		Logger:  logger,
	}
	require.NoError(t, err)

	command, err := NewNative(cfg, options)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, command.Start(ctx))
	require.NoError(t, command.Wait())
	require.Equal(t, expectedStdout, stdout.String())
	require.Equal(t, expectedStderr, stderr.String())
}

func testExecuteVirtualCommand(
	t *testing.T,
	source []byte,
	env []string,
	input io.Reader,
	expectedStdout string,
	logger *zap.Logger,
) {
	t.Helper()

	idResolver := identity.NewResolver(identity.AllLifecycleIdentity)

	doc := document.New(source, idResolver)
	node, err := doc.Root()
	require.NoError(t, err)

	blocks := document.CollectCodeBlocks(node)
	require.Len(t, blocks, 1)

	cfg, err := NewConfigFromCodeBlock(blocks[0])
	require.NoError(t, err)

	stdout := bytes.NewBuffer(nil)

	options := &VirtualCommandOptions{
		Session: MustNewSessionWithEnv(env...),
		Stdout:  stdout,
		Stdin:   input,
		Logger:  logger,
	}
	command, err := NewVirtual(cfg, options)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	require.NoError(t, command.Start(ctx))
	require.NoError(t, command.Wait())
	require.Equal(t, expectedStdout, stdout.String())
}

func TestCommandWithSession(t *testing.T) {
	setterCfg := &Config{
		ProgramName: "bash",
		Source: &runnerv2alpha1.ProgramConfig_Commands{
			Commands: &runnerv2alpha1.ProgramConfig_CommandList{
				Items: []string{"export TEST_ENV=test1"},
			},
		},
		Mode: runnerv2alpha1.CommandMode_COMMAND_MODE_INLINE,
	}
	getterCfg := &Config{
		ProgramName: "bash",
		Source: &runnerv2alpha1.ProgramConfig_Commands{
			Commands: &runnerv2alpha1.ProgramConfig_CommandList{
				Items: []string{"echo -n \"TEST_ENV equals $TEST_ENV\""},
			},
		},
		Mode: runnerv2alpha1.CommandMode_COMMAND_MODE_INLINE,
	}

	t.Run("Native", func(t *testing.T) {
		t.Parallel()

		sess := NewSession()

		commandSetter, err := NewNative(
			setterCfg,
			&NativeCommandOptions{
				Session: sess,
			},
		)
		require.NoError(t, err)
		require.NoError(t, commandSetter.Start(context.Background()))
		require.NoError(t, commandSetter.Wait())

		require.Equal(t, []string{"TEST_ENV=test1"}, sess.GetEnv())

		stdout := bytes.NewBuffer(nil)
		commandGetter, err := NewNative(
			getterCfg,
			&NativeCommandOptions{
				Session: sess,
				Stdout:  stdout,
			},
		)
		require.NoError(t, err)
		require.NoError(t, commandGetter.Start(context.Background()))
		require.NoError(t, commandGetter.Wait())
		require.Equal(t, "TEST_ENV equals test1", stdout.String())
	})

	t.Run("Virtual", func(t *testing.T) {
		t.Parallel()

		sess := NewSession()

		commandSetter, err := NewVirtual(
			setterCfg,
			&VirtualCommandOptions{
				Session: sess,
			},
		)
		require.NoError(t, err)
		require.NoError(t, commandSetter.Start(context.Background()))
		require.NoError(t, commandSetter.Wait())

		require.Equal(t, []string{"TEST_ENV=test1"}, sess.GetEnv())

		stdout := bytes.NewBuffer(nil)
		commandGetter, err := NewVirtual(
			getterCfg,
			&VirtualCommandOptions{
				Session: sess,
				Stdout:  stdout,
			},
		)
		require.NoError(t, err)
		require.NoError(t, commandGetter.Start(context.Background()))
		require.NoError(t, commandGetter.Wait())
		require.Equal(t, "TEST_ENV equals test1", stdout.String())
	})
}

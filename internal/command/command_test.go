//go:build !windows

package command

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/stateful/runme/v3/internal/document"
	"github.com/stateful/runme/v3/internal/document/identity"
	runnerv2alpha1 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/runner/v2alpha1"
)

var (
	testConfigBasicProgram = &ProgramConfig{
		ProgramName: "echo",
		Arguments:   []string{"-n", "test"},
		Mode:        runnerv2alpha1.CommandMode_COMMAND_MODE_INLINE,
	}

	testConfigInvalidProgram = &ProgramConfig{
		ProgramName: "invalidProgram",
		Source: &runnerv2alpha1.ProgramConfig_Commands{
			Commands: &runnerv2alpha1.ProgramConfig_CommandList{
				Items: []string{"echo -n test"},
			},
		},
		Mode: runnerv2alpha1.CommandMode_COMMAND_MODE_INLINE,
	}
)

func init() {
	EnvDumpCommand = "env -0"
}

func TestCommandFromCodeBlocks(t *testing.T) {
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
		Logger: zaptest.NewLogger(t),
		Stdout: stdout,
		Stderr: stderr,
		Stdin:  input,
	}

	command := NewFactory(nil, nil).Build(cfg, options)

	require.NoError(t, command.Start(context.Background()))
	assert.NoError(t, command.Wait())
	assert.Equal(t, expectedStdout, stdout.String())
	assert.Equal(t, expectedStderr, stderr.String())
}

func newSessionWithEnv(env ...string) (*Session, error) {
	s := NewSession()
	if err := s.SetEnv(env...); err != nil {
		return nil, err
	}
	return s, nil
}

func mustNewSessionWithEnv(env ...string) *Session {
	s, err := newSessionWithEnv(env...)
	if err != nil {
		panic(err)
	}
	return s
}

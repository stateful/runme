//go:build !windows
// +build !windows

package command

import (
	"bytes"
	"context"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/stateful/runme/v3/internal/command/testdata"
	"github.com/stateful/runme/v3/internal/session"
	runnerv2 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/runner/v2"
)

func TestInlineShellCommand_CollectEnv(t *testing.T) {
	t.Parallel()

	t.Run("Fifo", func(t *testing.T) {
		envCollectorUseFifo = true
		testInlineShellCommandCollectEnv(t)
	})

	t.Run("KillCommandWhileUsingFifo", func(t *testing.T) {
		envCollectorUseFifo = true

		cfg := &ProgramConfig{
			ProgramName: "bash",
			Source: &runnerv2.ProgramConfig_Commands{
				Commands: &runnerv2.ProgramConfig_CommandList{
					Items: []string{
						"export TEST_ENV=1",
						"sleep 5",
					},
				},
			},
			Mode: runnerv2.CommandMode_COMMAND_MODE_INLINE,
		}
		sess, err := session.New()
		require.NoError(t, err)
		factory := NewFactory(WithLogger(zaptest.NewLogger(t)))

		command, err := factory.Build(cfg, CommandOptions{Session: sess})
		require.NoError(t, err)
		err = command.Start(context.Background())
		require.NoError(t, err)

		errC := make(chan error, 1)
		go func() {
			<-time.After(time.Second)
			errC <- command.Signal(os.Kill)
		}()
		err = <-errC
		require.NoError(t, err)

		err = command.Wait(context.Background())
		require.EqualError(t, err, "signal: killed")

		got, ok := sess.GetEnv("TEST_ENV")
		assert.False(t, ok)
		assert.Equal(t, "", got)
	})

	t.Run("NonFifo", func(t *testing.T) {
		envCollectorUseFifo = false
		testInlineShellCommandCollectEnv(t)
	})
}

func TestInlineShellCommand_MaxEnvSize(t *testing.T) {
	t.Parallel()

	sess, err := session.New()
	require.NoError(t, err)

	envName := "TEST"
	envValue := strings.Repeat("a", session.MaxEnvSizeInBytes-len(envName)-1) // -1 for the "=" sign
	err = sess.SetEnv(context.Background(), createEnv(envName, envValue))
	require.NoError(t, err)

	factory := NewFactory(
		WithLogger(zaptest.NewLogger(t)),
		WithRuntime(&hostRuntime{}), // stub runtime and do not include environ
	)
	cfg := &ProgramConfig{
		ProgramName: "bash",
		Source: &runnerv2.ProgramConfig_Commands{
			Commands: &runnerv2.ProgramConfig_CommandList{
				Items: []string{
					"echo -n $" + envName,
				},
			},
		},
		Mode: runnerv2.CommandMode_COMMAND_MODE_FILE,
	}
	stdout := bytes.NewBuffer(nil)
	command, err := factory.Build(cfg, CommandOptions{Session: sess, Stdout: stdout})
	require.NoError(t, err)

	err = command.Start(context.Background())
	require.NoError(t, err)
	err = command.Wait(context.Background())
	require.NoError(t, err)

	assert.Equal(t, envValue, stdout.String())
}

func TestInlineShellCommand_MaxEnvironSizeInBytes(t *testing.T) {
	t.Parallel()

	sess, err := session.New()
	require.NoError(t, err)

	// Set multiple environment variables of [MaxEnvSizeInBytes] length.
	// [StoreStdoutEnvName] is also set but it exceeds [MaxEnvironSizeInBytes],
	// however, it's allowed to be trimmed so it should not cause an error.
	envCount := math.Ceil(float64(session.MaxEnvironSizeInBytes) / float64(session.MaxEnvSizeInBytes))
	envValue := strings.Repeat("a", session.MaxEnvSizeInBytes-1) // -1 for the equal sign
	for i := 0; i < int(envCount); i++ {
		name := "TEST" + strconv.Itoa(i)
		value := envValue[:len(envValue)-len(name)]
		err := sess.SetEnv(context.Background(), createEnv(name, value))
		require.NoError(t, err)
	}
	err = sess.SetEnv(context.Background(), createEnv(StoreStdoutEnvName, envValue[:len(envValue)-len(StoreStdoutEnvName)]))
	require.NoError(t, err)

	factory := NewFactory(
		WithLogger(zaptest.NewLogger(t)),
		WithRuntime(&hostRuntime{}), // stub runtime and do not include environ
	)
	cfg := &ProgramConfig{
		ProgramName: "bash",
		Source: &runnerv2.ProgramConfig_Commands{
			Commands: &runnerv2.ProgramConfig_CommandList{
				Items: []string{
					"echo -n $" + StoreStdoutEnvName,
				},
			},
		},
		Mode: runnerv2.CommandMode_COMMAND_MODE_FILE,
	}
	command, err := factory.Build(cfg, CommandOptions{Session: sess})
	require.NoError(t, err)

	err = command.Start(context.Background())
	require.NoError(t, err)
	err = command.Wait(context.Background())
	require.NoError(t, err)
}

func TestInlineShellCommand_LargeOutput(t *testing.T) {
	t.Parallel()

	temp := t.TempDir()
	fileName := filepath.Join(temp, "large_output.json")
	_, err := testdata.UngzipToFile(testdata.Users1MGzip, fileName)
	require.NoError(t, err)

	factory := NewFactory(WithLogger(zaptest.NewLogger(t)))
	cfg := &ProgramConfig{
		ProgramName: "bash",
		Source: &runnerv2.ProgramConfig_Commands{
			Commands: &runnerv2.ProgramConfig_CommandList{
				Items: []string{
					"cat " + fileName,
				},
			},
		},
		Mode: runnerv2.CommandMode_COMMAND_MODE_INLINE,
	}
	sess, err := session.New(
		session.WithOwl(false),
		session.WithSeedEnv(os.Environ()),
	)
	require.NoError(t, err)

	stdout := bytes.NewBuffer(nil)
	command, err := factory.Build(cfg, CommandOptions{Session: sess, Stdout: stdout})
	require.NoError(t, err)

	err = command.Start(context.Background())
	require.NoError(t, err)
	err = command.Wait(context.Background())
	require.NoError(t, err)

	expected, err := os.ReadFile(fileName)
	require.NoError(t, err)
	got := stdout.Bytes()
	assert.EqualValues(t, expected, got)
}

func testInlineShellCommandCollectEnv(t *testing.T) {
	t.Helper()

	cfg := &ProgramConfig{
		ProgramName: "bash",
		Source: &runnerv2.ProgramConfig_Script{
			Script: "export TEST_ENV=1",
		},
		Mode: runnerv2.CommandMode_COMMAND_MODE_INLINE,
	}
	sess, err := session.New(session.WithSeedEnv(os.Environ()))
	require.NoError(t, err)

	testExecuteCommandWithSession(t, cfg, sess, nil, "", "")

	got, ok := sess.GetEnv("TEST_ENV")
	assert.True(t, ok)
	assert.Equal(t, "1", got)
}

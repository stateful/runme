//go:build !windows
// +build !windows

package command

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	runnerv2alpha1 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/runner/v2alpha1"
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
			Source: &runnerv2alpha1.ProgramConfig_Commands{
				Commands: &runnerv2alpha1.ProgramConfig_CommandList{
					Items: []string{
						"export TEST_ENV=1",
						"sleep 5",
					},
				},
			},
			Mode: runnerv2alpha1.CommandMode_COMMAND_MODE_INLINE,
		}
		sess := NewSession()
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

		err = command.Wait()
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

func testInlineShellCommandCollectEnv(t *testing.T) {
	t.Helper()

	cfg := &ProgramConfig{
		ProgramName: "bash",
		Source: &runnerv2alpha1.ProgramConfig_Script{
			Script: "export TEST_ENV=1",
		},
		Mode: runnerv2alpha1.CommandMode_COMMAND_MODE_INLINE,
	}
	sess := NewSession()

	testExecuteCommandWithSession(t, cfg, sess, nil, "", "")

	got, ok := sess.GetEnv("TEST_ENV")
	assert.True(t, ok)
	assert.Equal(t, "1", got)
}

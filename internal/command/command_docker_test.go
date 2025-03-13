//go:build docker_enabled

package command

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/runmedev/runme/v3/internal/dockerexec"
	runnerv2 "github.com/runmedev/runme/v3/pkg/api/gen/proto/go/runme/runner/v2"
)

func TestDockerCommand(t *testing.T) {
	t.Parallel()

	docker, err := dockerexec.New(&dockerexec.Options{Debug: false, Image: "alpine:3.19"})
	require.NoError(t, err)

	factory := NewFactory(WithDocker(docker), WithLogger(zaptest.NewLogger(t)))

	// This test case is treated as a warm up. Do not parallelize.
	t.Run("NoOutput", func(t *testing.T) {
		cmd, err := factory.Build(
			&ProgramConfig{
				ProgramName: "echo",
				Arguments:   []string{"-n", "test"},
				Interactive: true,
				Mode:        runnerv2.CommandMode_COMMAND_MODE_INLINE,
			},
			CommandOptions{},
		)
		require.NoError(t, err)
		require.NoError(t, cmd.Start(context.Background()))
		require.NoError(t, cmd.Wait(context.Background()))
	})

	t.Run("Output", func(t *testing.T) {
		t.Parallel()
		stdout := bytes.NewBuffer(nil)
		cmd, err := factory.Build(
			&ProgramConfig{
				ProgramName: "echo",
				Arguments:   []string{"-n", "test"},
				Interactive: true,
				Mode:        runnerv2.CommandMode_COMMAND_MODE_INLINE,
			},
			CommandOptions{Stdout: stdout},
		)
		require.NoError(t, err)
		require.NoError(t, cmd.Start(context.Background()))
		require.NoError(t, cmd.Wait(context.Background()))
		assert.Equal(t, "test", stdout.String())
	})

	t.Run("Running", func(t *testing.T) {
		t.Parallel()
		cmd, err := factory.Build(
			&ProgramConfig{
				ProgramName: "sleep",
				Arguments:   []string{"1"},
				Mode:        runnerv2.CommandMode_COMMAND_MODE_INLINE,
			},
			CommandOptions{},
		)
		require.NoError(t, err)
		require.NoError(t, cmd.Start(context.Background()))
		require.True(t, cmd.Running())
		require.Greater(t, cmd.Pid(), 0)
		require.NoError(t, cmd.Wait(context.Background()))
	})

	t.Run("NonZeroExit", func(t *testing.T) {
		t.Parallel()

		cmd, err := factory.Build(
			&ProgramConfig{
				ProgramName: "sh",
				Arguments:   []string{"-c", "exit 11"},
				Mode:        runnerv2.CommandMode_COMMAND_MODE_INLINE,
			},
			CommandOptions{},
		)
		require.NoError(t, err)
		require.NoError(t, cmd.Start(context.Background()))
		require.Error(t, cmd.Wait(context.Background()), "exit code 11 due to error \"\"")
	})
}

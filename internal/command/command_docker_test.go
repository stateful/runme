//go:build test_with_docker

package command

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/stateful/runme/v3/internal/config"
	"github.com/stateful/runme/v3/internal/dockerexec"
	runnerv2alpha1 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/runner/v2alpha1"
)

func TestDockerCommand(t *testing.T) {
	t.Parallel()

	docker, err := dockerexec.New(&dockerexec.Options{Debug: false, Image: "alpine:3.19"})
	require.NoError(t, err)

	factory := NewFactory(&config.Config{}, NewDockerRuntime(docker), zaptest.NewLogger(t))

	// This test case is treated as a warm up. Do not parallelize.
	t.Run("NoOutput", func(t *testing.T) {
		cmd := factory.Build(
			&ProgramConfig{
				ProgramName: "echo",
				Arguments:   []string{"-n", "test"},
				Interactive: true,
				Mode:        runnerv2alpha1.CommandMode_COMMAND_MODE_INLINE,
			},
			Options{},
		)
		require.NoError(t, cmd.Start(context.Background()))
		require.NoError(t, cmd.Wait())
	})

	t.Run("Output", func(t *testing.T) {
		t.Parallel()
		stdout := bytes.NewBuffer(nil)
		cmd := factory.Build(
			&ProgramConfig{
				ProgramName: "echo",
				Arguments:   []string{"-n", "test"},
				Interactive: true,
				Mode:        runnerv2alpha1.CommandMode_COMMAND_MODE_INLINE,
			},
			Options{Stdout: stdout},
		)
		require.NoError(t, cmd.Start(context.Background()))
		require.NoError(t, cmd.Wait())
		assert.Equal(t, "test", stdout.String())
	})

	t.Run("Running", func(t *testing.T) {
		t.Parallel()
		cmd := factory.Build(
			&ProgramConfig{
				ProgramName: "sleep",
				Arguments:   []string{"1"},
				Mode:        runnerv2alpha1.CommandMode_COMMAND_MODE_INLINE,
			},
			Options{},
		)
		err := cmd.Start(context.Background())
		require.NoError(t, err)
		require.True(t, cmd.Running())
		require.Greater(t, cmd.Pid(), 0)
		require.NoError(t, cmd.Wait())
	})

	t.Run("NonZeroExit", func(t *testing.T) {
		t.Skip("enable when [envCollector] supports [Runtime]")

		t.Parallel()

		cmd := factory.Build(
			&ProgramConfig{
				ProgramName: "sh",
				Arguments:   []string{"-c", "exit 11"},
				Mode:        runnerv2alpha1.CommandMode_COMMAND_MODE_INLINE,
			},
			Options{},
		)
		require.NoError(t, err)
		require.NoError(t, cmd.Start(context.Background()))
		// TODO(adamb): wait should return non-nil error due to non-zero exit code.
		require.NoError(t, cmd.Wait())
		require.Equal(t, 11, cmd.(*dockerCommand).cmd.ProcessState.ExitCode)
	})
}

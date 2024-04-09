//go:build test_with_docker

package command

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stateful/runme/v3/internal/dockerexec"
)

func TestDockerCommand(t *testing.T) {
	t.Parallel()

	dockerCmdFactory, err := dockerexec.New(&dockerexec.Options{Image: "alpine:3.19"})
	require.NoError(t, err)

	t.Run("Output", func(t *testing.T) {
		// Do not run in parallel; this is a warm up test.
		stdout := bytes.NewBuffer(nil)
		cmd := NewDocker(
			testConfigBasicProgram,
			dockerCmdFactory,
			Options{Stdout: stdout},
		)
		require.NoError(t, cmd.Start(context.Background()))
		require.NoError(t, cmd.Wait())
		assert.Equal(t, "test", stdout.String())
	})

	t.Run("NoOutput", func(t *testing.T) {
		t.Parallel()
		cmd := NewDocker(testConfigBasicProgram, dockerCmdFactory, Options{})
		require.NoError(t, cmd.Start(context.Background()))
		require.NoError(t, cmd.Wait())
	})

	t.Run("Running", func(t *testing.T) {
		t.Parallel()
		cmd := NewDocker(
			&Config{
				ProgramName: "sleep",
				Arguments:   []string{"5"},
			},
			dockerCmdFactory,
			Options{},
		)
		require.NoError(t, cmd.Start(context.Background()))
		require.True(t, cmd.Running())
		require.Greater(t, cmd.Pid(), 0)
		require.NoError(t, cmd.Wait())
	})

	t.Run("NonZeroExit", func(t *testing.T) {
		t.Parallel()
		cmd := NewDocker(
			&Config{
				ProgramName: "sh",
				Arguments:   []string{"-c", "exit 11"},
			},
			dockerCmdFactory,
			Options{},
		)
		require.NoError(t, err)
		require.NoError(t, cmd.Start(context.Background()))
		// TODO(adamb): wait should return non-nil error due to non-zero exit code.
		require.NoError(t, cmd.Wait())
		require.Equal(t, 11, cmd.cmd.ProcessState.ExitCode)
	})
}

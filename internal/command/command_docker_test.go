//go:build test_with_docker

package command

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stateful/runme/v3/internal/dockercmd"
)

func TestDockerCommand(t *testing.T) {
	t.Parallel()

	t.Run("OptionsIsNil", func(t *testing.T) {
		_, err := NewDocker(testConfigBasicProgram, nil)
		require.EqualError(t, err, "options cannot be nil")
	})

	dockerCmdFactory, err := dockercmd.New(&dockercmd.Options{Image: "alpine:3.19"})
	require.NoError(t, err)

	t.Run("Output", func(t *testing.T) {
		// Do not run in parallel; this is a warm up test.
		stdout := bytes.NewBuffer(nil)
		opts := &DockerCommandOptions{
			CmdFactory: dockerCmdFactory,
			Stdout:     stdout,
		}
		cmd, err := NewDocker(testConfigBasicProgram, opts)
		require.NoError(t, err)
		require.NoError(t, cmd.Start(context.Background()))
		require.NoError(t, cmd.Wait())
		assert.Equal(t, "test", stdout.String())
	})

	t.Run("NoOutput", func(t *testing.T) {
		t.Parallel()
		opts := &DockerCommandOptions{
			CmdFactory: dockerCmdFactory,
		}
		cmd, err := NewDocker(testConfigBasicProgram, opts)
		require.NoError(t, err)
		require.NoError(t, cmd.Start(context.Background()))
		require.NoError(t, cmd.Wait())
	})

	t.Run("Running", func(t *testing.T) {
		t.Parallel()
		opts := &DockerCommandOptions{
			CmdFactory: dockerCmdFactory,
		}
		cmd, err := NewDocker(
			&Config{
				ProgramName: "sleep",
				Arguments:   []string{"5"},
			},
			opts,
		)
		require.NoError(t, err)
		require.NoError(t, cmd.Start(context.Background()))
		require.True(t, cmd.Running())
		require.Greater(t, cmd.Pid(), 0)
		require.NoError(t, cmd.Wait())
	})

	t.Run("NonZeroExit", func(t *testing.T) {
		t.Parallel()
		opts := &DockerCommandOptions{
			CmdFactory: dockerCmdFactory,
		}
		cmd, err := NewDocker(
			&Config{
				ProgramName: "sh",
				Arguments:   []string{"-c", "exit 11"},
			},
			opts,
		)
		require.NoError(t, err)
		require.NoError(t, cmd.Start(context.Background()))
		// TODO(adamb): wait should return non-nil error due to non-zero exit code.
		require.NoError(t, cmd.Wait())
		require.Equal(t, 11, cmd.cmd.ProcessState.ExitCode)
	})
}

//go:build !windows

package command

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVirtualCommand(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		cmd := newVirtual(testConfigBasicProgram, Options{})
		require.NoError(t, cmd.Start(context.Background()))
		require.NoError(t, cmd.Wait())
	})

	t.Run("Output", func(t *testing.T) {
		stdout := bytes.NewBuffer(nil)
		cmd := newVirtual(testConfigBasicProgram, Options{Stdout: stdout})
		require.NoError(t, cmd.Start(context.Background()))
		require.NoError(t, cmd.Wait())
		assert.Equal(t, "test", stdout.String())
	})

	t.Run("Getters", func(t *testing.T) {
		cmd := newVirtual(
			&ProgramConfig{
				ProgramName: "sleep",
				Arguments:   []string{"1"},
			},
			Options{},
		)
		require.NoError(t, cmd.Start(context.Background()))

		require.True(t, cmd.Running())
		require.Greater(t, cmd.Pid(), 1)
		require.NoError(t, cmd.Wait())
	})
}

//go:build !windows

package command

import (
	"bytes"
	"context"
	"os"
	"testing"

	runnerv2alpha1 "github.com/stateful/runme/internal/gen/proto/go/runme/runner/v2alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	// Set to false to disable sending signals to process groups in tests.
	// This can be turned on if setSysProcAttrPgid() is called in Start().
	SignalToProcessGroup = false
}

func TestNativeCommand(t *testing.T) {
	t.Run("OptionsIsNil", func(t *testing.T) {
		cmd, err := NewNative(testConfigBasicProgram, nil)
		require.NoError(t, err)
		require.NoError(t, cmd.Start(context.Background()))
		require.NoError(t, cmd.Wait())
	})

	t.Run("Output", func(t *testing.T) {
		stdout := bytes.NewBuffer(nil)
		opts := &NativeCommandOptions{
			Stdout: stdout,
		}
		cmd, err := NewNative(testConfigBasicProgram, opts)
		require.NoError(t, err)
		require.NoError(t, cmd.Start(context.Background()))
		require.NoError(t, cmd.Wait())
		assert.Equal(t, "test", stdout.String())
	})

	t.Run("Shell", func(t *testing.T) {
		stdout := bytes.NewBuffer(nil)
		opts := &NativeCommandOptions{
			Stdout: stdout,
		}
		cmd, err := NewNative(testConfigShellProgram, opts)
		require.NoError(t, err)
		require.NoError(t, cmd.Start(context.Background()))
		require.NoError(t, cmd.Wait())
		assert.Equal(t, "test", stdout.String())
	})

	t.Run("DevStdin", func(t *testing.T) {
		cmd, err := NewNative(testConfigShellProgram, &NativeCommandOptions{Stdin: os.Stdin})
		require.NoError(t, err)
		require.NoError(t, cmd.Start(context.Background()))
		require.NoError(t, cmd.Wait())
	})
}

func TestNativeCommandStopWithSignal(t *testing.T) {
	cfg := &Config{
		ProgramName: "sleep",
		Arguments:   []string{"10"},
		Mode:        runnerv2alpha1.CommandMode_COMMAND_MODE_INLINE,
	}

	t.Run("SIGINT", func(t *testing.T) {
		cmd, err := NewNative(cfg, nil)
		require.NoError(t, err)
		require.NoError(t, cmd.Start(context.Background()))

		errc := make(chan error, 1)
		go func() {
			errc <- cmd.StopWithSignal(os.Interrupt)
		}()

		require.EqualError(t, cmd.Wait(), "signal: interrupt")
		require.NoError(t, <-errc)
	})

	t.Run("SIGKILL", func(t *testing.T) {
		cmd, err := NewNative(cfg, nil)
		require.NoError(t, err)
		require.NoError(t, cmd.Start(context.Background()))

		errc := make(chan error, 1)
		go func() {
			errc <- cmd.StopWithSignal(os.Kill)
		}()

		require.EqualError(t, cmd.Wait(), "signal: killed")
		require.NoError(t, <-errc)
	})
}

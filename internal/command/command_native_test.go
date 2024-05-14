//go:build !windows

package command

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	runnerv2alpha1 "github.com/stateful/runme/v3/pkg/gen/proto/go/runme/runner/v2alpha1"
)

func init() {
	// Set to false to disable sending signals to process groups in tests.
	// This can be turned on if setSysProcAttrPgid() is called in Start().
	SignalToProcessGroup = false
}

func TestNativeCommand(t *testing.T) {
	t.Run("OptionsIsNil", func(t *testing.T) {
		cmd := NewNative(testConfigBasicProgram, Options{})
		require.NoError(t, cmd.Start(context.Background()))
		require.NoError(t, cmd.Wait())
	})

	t.Run("Output", func(t *testing.T) {
		stdout := bytes.NewBuffer(nil)
		cmd := NewNative(testConfigBasicProgram, Options{Stdout: stdout})
		require.NoError(t, cmd.Start(context.Background()))
		require.NoError(t, cmd.Wait())
		assert.Equal(t, "test", stdout.String())
	})

	t.Run("Shell", func(t *testing.T) {
		stdout := bytes.NewBuffer(nil)
		cmd := NewNative(testConfigShellProgram, Options{Stdout: stdout})
		require.NoError(t, cmd.Start(context.Background()))
		require.NoError(t, cmd.Wait())
		assert.Equal(t, "test", stdout.String())
	})

	t.Run("DevStdin", func(t *testing.T) {
		cmd := NewNative(testConfigShellProgram, Options{Stdin: os.Stdin})
		require.NoError(t, cmd.Start(context.Background()))
		require.NoError(t, cmd.Wait())
	})

	t.Run("Invalid", func(t *testing.T) {
		stdout := bytes.NewBuffer(nil)
		cmd := NewNative(testConfigInvalidProgram, Options{Stdout: stdout})
		err := cmd.Start(context.Background())
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed program lookup \"invalidProgram\"")
		assert.Equal(t, "", stdout.String())
	})

	t.Run("Default to cat", func(t *testing.T) {
		stdout := bytes.NewBuffer(nil)
		cmd := NewNative(testConfigDefaultToCat, Options{Stdout: stdout})
		require.NoError(t, cmd.Start(context.Background()))
		require.NoError(t, cmd.Wait())
		assert.Equal(t, "SELECT * FROM users;", stdout.String())
	})
}

func TestNativeCommandStopWithSignal(t *testing.T) {
	cfg := &Config{
		ProgramName: "sleep",
		Arguments:   []string{"10"},
		Mode:        runnerv2alpha1.CommandMode_COMMAND_MODE_INLINE,
	}

	t.Run("SIGINT", func(t *testing.T) {
		cmd := NewNative(cfg, Options{})
		require.NoError(t, cmd.Start(context.Background()))

		errc := make(chan error, 1)
		go func() {
			errc <- cmd.Signal(os.Interrupt)
		}()

		require.EqualError(t, cmd.Wait(), "signal: interrupt")
		require.NoError(t, <-errc)
	})

	t.Run("SIGKILL", func(t *testing.T) {
		cmd := NewNative(cfg, Options{})
		require.NoError(t, cmd.Start(context.Background()))

		errc := make(chan error, 1)
		go func() {
			errc <- cmd.Signal(os.Kill)
		}()

		require.EqualError(t, cmd.Wait(), "signal: killed")
		require.NoError(t, <-errc)
	})
}

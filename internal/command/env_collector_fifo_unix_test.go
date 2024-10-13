//go:build !windows
// +build !windows

package command

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_envCollectorFifo(t *testing.T) {
	t.Parallel()

	collector, err := newEnvCollectorFifo(scanEnv, nil, nil)
	require.NoError(t, err)

	err = os.WriteFile(collector.prePath(), []byte("ENV_1=1"), 0o600)
	require.NoError(t, err)
	err = os.WriteFile(collector.postPath(), []byte("ENV_2=2"), 0o600)
	require.NoError(t, err)

	t.Run("ExtraEnv", func(t *testing.T) {
		require.Len(t, collector.ExtraEnv(), 2)
	})

	t.Run("SetOnShell", func(t *testing.T) {
		buf := new(bytes.Buffer)
		err := collector.SetOnShell(buf)
		require.NoError(t, err)
		expected := " env -0 > " + collector.prePath() + "\n" +
			" __cleanup() {\nrv=$?\nenv -0 > " + collector.postPath() + "\nexit $rv\n}\n" +
			" trap -- \"__cleanup\" EXIT\n"
		require.Equal(t, expected, buf.String())
	})

	t.Run("Diff", func(t *testing.T) {
		changedEnv, deletedEnv, err := collector.Diff()
		require.NoError(t, err)
		require.Equal(t, []string{"ENV_2=2"}, changedEnv)
		require.Equal(t, []string{"ENV_1"}, deletedEnv)
	})
}

func Test_envCollectorFifo_WithoutWriter(t *testing.T) {
	t.Parallel()

	collector, err := newEnvCollectorFifo(scanEnv, nil, nil)
	require.NoError(t, err)

	_, _, err = collector.Diff()
	require.NoError(t, err)
}

package command

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnvCollectorFile(t *testing.T) {
	t.Parallel()

	collector, err := newEnvCollectorFile(scanEnv, nil, nil)
	require.NoError(t, err)

	err = os.WriteFile(collector.prePath(), []byte("ENV_1=1"), 0o600)
	require.NoError(t, err)
	err = os.WriteFile(collector.postPath(), []byte("ENV_2=2"), 0o600)
	require.NoError(t, err)

	changedEnv, deletedEnv, err := collector.Diff()
	require.NoError(t, err)
	require.Equal(t, []string{"ENV_2=2"}, changedEnv)
	require.Equal(t, []string{"ENV_1"}, deletedEnv)
}

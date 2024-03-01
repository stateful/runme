package owl

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Store(t *testing.T) {
	// t.Parallel()

	t.Run("overlay", func(t *testing.T) {
		envs := os.Environ()

		raw := []byte(`WASI_SDK_PATH=The path to the wasi-sdk directory # Path!`)
		store, err := NewStore(WithEnvs(envs...), WithSpecFile(".env.example", raw))
		require.NoError(t, err)

		require.Len(t, store.opSets, 2)
		require.Len(t, store.opSets[0].items, len(envs))
		require.Len(t, store.opSets[1].items, 1)

		err = store.Snapshot()
		require.NoError(t, err)
	})

	t.Run("load raw env", func(t *testing.T) {
		rawEnv, err := os.ReadFile("../../pkg/project/test_project/.env")
		require.NoError(t, err)
		rawEnvLocal, err := os.ReadFile("../../pkg/project/test_project/.env.local")
		require.NoError(t, err)

		store, err := NewStore(
			WithSpecFile(".env", rawEnv),
			WithSpecFile(".env.local", rawEnvLocal),
		)
		require.NoError(t, err)

		require.Len(t, store.opSets, 2)
		require.Len(t, store.opSets[0].items, 2)
		require.Len(t, store.opSets[1].items, 2)

		err = store.Snapshot()
		require.NoError(t, err)
	})
}

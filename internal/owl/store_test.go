package owl

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Store(t *testing.T) {
	t.Parallel()

	t.Run("overlay", func(t *testing.T) {
		envs := os.Environ()

		raw := []byte(`WASI_SDK_PATH=The path to the wasi-sdk directory # Path!`)
		store, err := NewStore(WithEnvs(envs...), WithSpecFile(".env.example", raw))
		require.NoError(t, err)

		require.Len(t, store.opSets, 2)
		require.Len(t, store.opSets[0].items, len(envs))
		require.Len(t, store.opSets[1].items, 1)

		snapshot, err := store.Snapshot()
		require.NoError(t, err)
		require.EqualValues(t, "Path", snapshot[0].Spec.Name)
	})

	t.Run("load raw env", func(t *testing.T) {
		// todo(sebastian): needs better solution
		rawEnvLocal, err := os.ReadFile("../../pkg/project/test_project/.env.local")
		require.NoError(t, err)
		rawEnv, err := os.ReadFile("../../pkg/project/test_project/.env")
		require.NoError(t, err)

		store, err := NewStore(
			// order matters
			WithEnvFile(".env.local", rawEnvLocal),
			WithEnvFile(".env", rawEnv),
		)
		require.NoError(t, err)

		require.Len(t, store.opSets, 2)
		require.Len(t, store.opSets[0].items, 2)
		require.Len(t, store.opSets[1].items, 2)

		snapshot, err := store.Snapshot()
		require.NoError(t, err)
		require.Len(t, snapshot, 3)

		require.EqualValues(t, "secret1_overridden", snapshot[0].Value.Resolved)
		require.EqualValues(t, "secret2", snapshot[1].Value.Resolved)
		require.EqualValues(t, "secret3", snapshot[2].Value.Resolved)
	})
}

package owl

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Store(t *testing.T) {
	t.Parallel()

	t.Run("load raw env", func(t *testing.T) {
		store, err := NewStore(WithSpecFile("../../pkg/project/test_project/.env"))
		require.NoError(t, err)
		require.Len(t, store.opSets, 1)
		require.Len(t, store.opSets[0].items, 2)
	})
}

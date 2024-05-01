package config

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestNewLoader(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() {
		NewLoader("", "yaml", fstest.MapFS{})
	}, "config name is not set")
}

func TestLoader_RootConfig(t *testing.T) {
	t.Parallel()

	t.Run("without root config", func(t *testing.T) {
		t.Parallel()

		fsys := fstest.MapFS{}
		loader := NewLoader("runme", "yaml", fsys, WithLogger(zaptest.NewLogger(t)))
		result, err := loader.RootConfig()
		require.ErrorIs(t, err, ErrRootConfigNotFound)
		require.Nil(t, result)
	})

	t.Run("with root config", func(t *testing.T) {
		t.Parallel()

		data := []byte("version: v1alpha1\n")
		fsys := fstest.MapFS{
			"runme.yaml": {
				Data: data,
			},
		}
		loader := NewLoader("runme", "yaml", fsys, WithLogger(zaptest.NewLogger(t)))
		result, err := loader.RootConfig()
		require.NoError(t, err)
		require.Equal(t, data, result)
	})
}

func TestLoader_ChainConfigs(t *testing.T) {
	t.Parallel()

	t.Run("without root config", func(t *testing.T) {
		t.Parallel()

		fsys := fstest.MapFS{}
		loader := NewLoader("runme", "yaml", fsys, WithLogger(zaptest.NewLogger(t)))
		result, err := loader.FindConfigChain("")
		require.NoError(t, err)
		require.Nil(t, result)
	})

	fsys := fstest.MapFS{
		"runme.yaml": {
			Data: []byte("path:runme.yaml"),
		},
		"nested/runme.yaml": {
			Data: []byte("path:nested/runme.yaml"),
		},
		"nested/path/runme.yaml": {
			Data: []byte("path:nested/path/runme.yaml"),
		},
		"other/runme.yaml": {
			Data: []byte("path:other/runme.yaml"),
		},
		"without/config": {
			Data: []byte("path:without/config"),
			Mode: fs.ModeDir,
		},
	}
	loader := NewLoader("runme", "yaml", fsys, WithLogger(zaptest.NewLogger(t)))

	t.Run("root config", func(t *testing.T) {
		result, err := loader.FindConfigChain("")
		require.NoError(t, err)
		require.Equal(
			t,
			[][]byte{[]byte("path:runme.yaml")},
			result,
		)
	})

	t.Run("nested config", func(t *testing.T) {
		result, err := loader.FindConfigChain("nested")
		require.NoError(t, err)
		require.Equal(
			t,
			[][]byte{[]byte("path:runme.yaml"), []byte("path:nested/runme.yaml")},
			result,
		)
	})

	t.Run("nested deep config", func(t *testing.T) {
		result, err := loader.FindConfigChain("nested/path")
		require.NoError(t, err)
		require.Equal(
			t,
			[][]byte{
				[]byte("path:runme.yaml"),
				[]byte("path:nested/runme.yaml"),
				[]byte("path:nested/path/runme.yaml"),
			},
			result,
		)
	})

	t.Run("nested without config", func(t *testing.T) {
		result, err := loader.FindConfigChain("without/config")
		require.NoError(t, err)
		require.Equal(
			t,
			[][]byte{[]byte("path:runme.yaml")},
			result,
		)
	})
}

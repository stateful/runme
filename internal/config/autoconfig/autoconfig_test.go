package autoconfig

import (
	"fmt"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"

	"github.com/stateful/runme/v3/internal/config"
)

func TestInvokeConfig(t *testing.T) {
	// Create fake filesystem and set it in the config loader.
	fsys := fstest.MapFS{
		"README.md": {
			Data: []byte("Hello, World!"),
		},
		"runme.yaml": {
			Data: []byte(fmt.Sprintf("version: v1alpha1\nproject:\n  filename: %s\n", "README.md")),
		},
	}

	err := Invoke(func(loader *config.Loader) error {
		loader.SetConfigRootPath(fsys)
		return nil
	})
	require.NoError(t, err)

	err = Invoke(func(
		*config.Config,
	) error {
		return nil
	})
	require.NoError(t, err)
}

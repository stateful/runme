package autoconfig

import (
	"fmt"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"

	"github.com/stateful/runme/v3/internal/config"
)

func TestInvokeForCommand_Config(t *testing.T) {
	// Create a fake filesystem and set it in [config.Loader].
	err := InvokeForCommand(func(loader *config.Loader) error {
		fsys := fstest.MapFS{
			"README.md": {
				Data: []byte("Hello, World!"),
			},
			"runme.yaml": {
				Data: []byte(fmt.Sprintf("version: v1alpha1\nproject:\n  filename: %s\n", "README.md")),
			},
		}
		loader.SetConfigRootPath(fsys)
		return nil
	})
	require.NoError(t, err)

	err = InvokeForCommand(func(
		*config.Config,
	) error {
		return nil
	})
	require.NoError(t, err)
}

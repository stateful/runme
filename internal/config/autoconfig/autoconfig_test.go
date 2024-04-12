package autoconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/stateful/runme/v3/internal/command"
	"github.com/stateful/runme/v3/internal/config"
	"github.com/stateful/runme/v3/internal/project"
)

func TestInvokeAll(t *testing.T) {
	tempDir := t.TempDir()
	readmeFilePath := filepath.Join(tempDir, "README.md")

	// Create a README.md file in the temp directory.
	err := os.WriteFile(readmeFilePath, []byte("Hello, World!"), 0o600)
	require.NoError(t, err)

	// Create a runme.yaml using the README.md file from above.
	// This won't work with the project as it requires the project
	// to be a subdirectory of the current working directory.
	configYAML := fmt.Sprintf("version: v1alpha1\ncore:\n  filename: %s\n", readmeFilePath)

	fmt.Println(string(configYAML))

	// Create a runme.yaml file in the temp directory.
	err = os.WriteFile(filepath.Join(tempDir, "/runme.yaml"), []byte(configYAML), 0o600)
	require.NoError(t, err)
	// And add it to the viper configuration.
	// It's ok as viper has no other dependencies
	// so nothing will be instantiated before
	// the configuration is loaded.
	err = Invoke(func(v *viper.Viper) {
		v.AddConfigPath(tempDir)
	})
	require.NoError(t, err)

	// Load all dependencies.
	err = Invoke(func(
		*config.Config,
		*zap.Logger,
		*project.Project,
		[]project.Filter,
		*command.Session,
		*viper.Viper,
	) error {
		return nil
	})
	require.NoError(t, err)
}

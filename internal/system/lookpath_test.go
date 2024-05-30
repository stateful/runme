//go:build unix

// TODO(adamb): remove the build flag when [LookPathUsingPathEnv] is implemented for Windows.

package system

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLookPath(t *testing.T) {
	tmp := t.TempDir()
	myBinaryPath := filepath.Join(tmp, "my-binary")

	// Create an empty file with execute permission.
	err := os.WriteFile(myBinaryPath, []byte{}, 0o111)
	require.NoError(t, err)

	path, err := LookPathUsingPathEnv("my-binary", tmp)
	require.NoError(t, err)
	require.Equal(t, myBinaryPath, path)
}

//go:build unix

// TODO(adamb): remove the build flag when [System.LookPath] is implemented for Windows.

package system

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLookPath(t *testing.T) {
	tmp := t.TempDir()
	myBinaryPath := filepath.Join(tmp, "my-binary")

	// Create an empty file with execute permission.
	err := os.WriteFile(myBinaryPath, []byte{}, 0o111)
	require.NoError(t, err)

	s := New(
		WithPathEnvGetter(func() string { return tmp }),
	)
	path, err := s.LookPath("my-binary")
	require.NoError(t, err)
	assert.Equal(t, myBinaryPath, path)
}

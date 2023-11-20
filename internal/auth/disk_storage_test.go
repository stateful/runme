package auth

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDiskStorage(t *testing.T) {
	tmpDir := t.TempDir()
	expectedToken := Token{Data: "data"}
	ds := DiskStorage{
		Location: filepath.Join(tmpDir, "more", "depth"),
	}

	// Happy path.
	err := ds.Save("token", expectedToken)
	require.NoError(t, err)

	var token Token
	err = ds.Load("token", &token)
	require.NoError(t, err)
	require.Equal(t, expectedToken, token)

	// Read from non existing key.
	err = ds.Load("invalid-key", &token)
	require.ErrorIs(t, err, ErrNotFound)

	// Read from invalid path.
	ds = DiskStorage{Location: "/path/does/not/exist"}
	err = ds.Load("token", &token)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrNotFound)
}

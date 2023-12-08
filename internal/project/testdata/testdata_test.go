package testdata

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDirProjectPath(t *testing.T) {
	path := DirProjectPath()
	require.True(t, strings.HasSuffix(path, filepath.Join("testdata", "dir-project")))
}

func TestGitProjectPath(t *testing.T) {
	path := GitProjectPath()
	require.True(t, strings.HasSuffix(path, filepath.Join("testdata", "git-project")))
}

func TestProjectFilePath(t *testing.T) {
	path := ProjectFilePath()
	require.True(t, strings.HasSuffix(path, filepath.Join("testdata", "file-project.md")))
}

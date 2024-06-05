package teststub

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/otiai10/copy"
	"github.com/stretchr/testify/require"
)

func Setup(t *testing.T, temp string) TestData {
	t.Helper()

	testDataSrc := originalTestDataPath()
	require.NoError(t, copy.Copy(testDataSrc, temp))

	err := os.Rename(
		filepath.Join(temp, "git-project", ".git.bkp"),
		filepath.Join(temp, "git-project", ".git"),
	)
	require.NoError(t, err)

	err = os.Rename(
		filepath.Join(temp, "git-project", ".gitignore.bkp"),
		filepath.Join(temp, "git-project", ".gitignore"),
	)
	require.NoError(t, err)

	err = os.Rename(
		filepath.Join(temp, "git-project", "nested", ".gitignore.bkp"),
		filepath.Join(temp, "git-project", "nested", ".gitignore"),
	)
	require.NoError(t, err)

	return TestData{root: temp}
}

type TestData struct {
	root string
}

func (d TestData) Root() string {
	return d.root
}

func (d TestData) Join(elems ...string) string {
	elems = append([]string{d.root}, elems...)
	return filepath.Join(elems...)
}

func (d TestData) DirProjectPath() string {
	return d.Join("dir-project")
}

func (d TestData) GitProjectPath() string {
	return d.Join("git-project")
}

func (d TestData) GitProjectNestedPath() string {
	return d.Join("git-project", "nested")
}

func (d TestData) ProjectFilePath() string {
	return d.Join("file-project.md")
}

func originalTestDataPath() string {
	_, b, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(b), "..", "testdata")
}

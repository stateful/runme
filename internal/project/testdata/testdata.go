package testdata

import (
	"path/filepath"
	"runtime"
)

func TestdataPath() string {
	return testdataDir()
}

func DirProjectPath() string {
	return filepath.Join(testdataDir(), "dir-project")
}

func GitProjectPath() string {
	return filepath.Join(testdataDir(), "git-project")
}

func GitProjectNestedPath() string {
	return filepath.Join(testdataDir(), "git-project", "nested")
}

func ProjectFilePath() string {
	return filepath.Join(testdataDir(), "file-project.md")
}

// TODO(adamb): a better approach is to store "testdata" during build time.
func testdataDir() string {
	_, b, _, _ := runtime.Caller(0)
	return filepath.Dir(b)
}

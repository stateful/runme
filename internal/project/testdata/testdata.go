package testdata

import (
	"log"
	"os"
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

func AssertGitProject() {
	assertGitProject()
}

// assertGitProject checks that ./testdata/git-project is a valid git project.
// If it's not it will fail with a call to action to run the right make targets.
func assertGitProject() {
	dir := GitProjectPath()

	gitProjectDestFiles := []string{
		filepath.Join(dir, ".git"),
		filepath.Join(dir, ".gitignore"),
		filepath.Join(dir, "nested", ".gitignore"),
	}

	for _, dest := range gitProjectDestFiles {
		if _, err := os.Stat(dest); err != nil {
			log.Fatalf("failed to assert %s: %v; please run maket target 'test/prepare-git-project'.", dest, err)
		}
	}
}

package testdata

import (
	"log"
	"os"
	"path/filepath"
)

func TestdataPath() string {
	return testdataPath()
}

func DirProjectPath() string {
	return filepath.Join(testdataPath(), "dir-project")
}

func GitProjectPath() string {
	dir := filepath.Join(testdataPath(), "git-project")
	assertGitProject(dir)
	return dir
}

func GitProjectNestedPath() string {
	dir := filepath.Join(testdataPath(), "git-project")
	assertGitProject(dir)
	return filepath.Join(dir, "nested")
}

func ProjectFilePath() string {
	return filepath.Join(testdataPath(), "file-project.md")
}

func testdataPath() string {
	path := os.Getenv("RUNME_TESTDATA_PATH")
	if path == "" {
		log.Fatalf("RUNME_TESTDATA_PATH is not set")
	}
	return path
}

// assertGitProject checks that $(RUNME_TESTDATA_PATH)/git-project is a valid git project.
// If it's not it will fail with a call to action to run the right make targets.
func assertGitProject(dir string) {
	gitProjectDestFiles := []string{
		filepath.Join(dir, ".git"),
		filepath.Join(dir, ".gitignore"),
		filepath.Join(dir, "nested", ".gitignore"),
	}

	for _, dest := range gitProjectDestFiles {
		if _, err := os.Stat(dest); err != nil {
			log.Fatalf("failed to assert %s: %v; please use make target 'test' or 'test/prepare-git-project'", dest, err)
		}
	}
}

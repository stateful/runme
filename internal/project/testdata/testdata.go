package testdata

import (
	"log"
	"os/exec"
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

func ProjectFilePath() string {
	return filepath.Join(testdataDir(), "file-project.md")
}

// TODO(adamb): a better approach is to store "testdata" during build time.
func testdataDir() string {
	_, b, _, _ := runtime.Caller(0)
	return filepath.Dir(b)
}

func PrepareGitProject() {
	prepareGitProject()
}

func CleanupGitProject() {
	cleanupGitProject()
}

// prepareGitProject copies .git.bkp from the ./testdata/git-project to .git in order to
// make ./testdata/git-project a valid git project.
func prepareGitProject() {
	cleanupGitProject()

	dir := GitProjectPath()

	srcBkpFilesToDestFiles := map[string]string{
		filepath.Join(dir, ".git.bkp"):                 filepath.Join(dir, ".git"),
		filepath.Join(dir, ".gitignore.bkp"):           filepath.Join(dir, ".gitignore"),
		filepath.Join(dir, "nested", ".gitignore.bkp"): filepath.Join(dir, "nested", ".gitignore"),
	}

	for src, dest := range srcBkpFilesToDestFiles {
		cmd := exec.Command("cp", "-f", "-r", src, dest)
		if output, err := cmd.CombinedOutput(); err != nil {
			log.Fatalf("failed to prepare %s: %v; output: %s", dest, err, output)
		}
	}
}

func cleanupGitProject() {
	dir := GitProjectPath()

	files := []string{
		filepath.Join(dir, ".git"),
		filepath.Join(dir, ".gitignore"),
		filepath.Join(dir, "nested", ".gitignore"),
	}

	for _, file := range files {
		cmd := exec.Command("rm", "-r", "-f", file)
		if output, err := cmd.CombinedOutput(); err != nil {
			log.Fatalf("failed clean up %s: %v; output: %s", file, err, output)
		}
	}
}

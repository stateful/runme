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
	dir := GitProjectPath()

	cmd := exec.Command("cp", "-fr", filepath.Join(dir, ".git.bkp"), filepath.Join(dir, ".git"))
	if output, err := cmd.CombinedOutput(); err != nil {
		log.Fatalf("failed to prepare .git: %v; output: %s", err, output)
	}

	cmd = exec.Command("cp", "-f", filepath.Join(dir, ".gitignore.bkp"), filepath.Join(dir, ".gitignore"))
	if output, err := cmd.CombinedOutput(); err != nil {
		log.Fatalf("failed to prepare .gitignore: %v; output: %s", err, output)
	}
}

func cleanupGitProject() {
	dir := GitProjectPath()

	cmd := exec.Command("rm", "-rf", filepath.Join(dir, ".git"))
	if output, err := cmd.CombinedOutput(); err != nil {
		log.Fatalf("failed clean up .git: %v; output: %s", err, output)
	}

	cmd = exec.Command("rm", "-f", filepath.Join(dir, ".gitignore"))
	if output, err := cmd.CombinedOutput(); err != nil {
		log.Fatalf("failed clean up .gitignore: %v; output: %s", err, output)
	}
}

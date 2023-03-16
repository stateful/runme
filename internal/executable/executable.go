package executable

import (
	"os"
	"path/filepath"
	"runtime"
)

var runmePath string

func InitExecutablePath() {
	execPath, _ := os.Executable()
	runmePath = execPath
}

func GetRunmeExecutablePath() string {
	if runmePath != "" {
		return runmePath
	}

	// if not initialized, assume we are in testing environment
	cwd, _ := os.Getwd()
	var res string

	for cwd != filepath.Base(cwd) && len(cwd) > 1 {
		if info, err := os.Stat(filepath.Join(cwd, "go.mod")); err == nil && !info.IsDir() {
			res = cwd
			break
		}

		cwd = filepath.Dir(cwd)
	}

	execFile := "runme"
	if runtime.GOOS == "windows" {
		execFile = "runme.exe"
	}

	if res != "" {
		runmePath = filepath.Join(res, execFile)
		return runmePath
	}

	return ""
}

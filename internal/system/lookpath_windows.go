package system

import "os/exec"

func lookPath(_, file string) (string, error) {
	// TODO(adamb): implement this for Windows.
	// Check out https://github.com/golang/go/blob/master/src/os/exec/lp_windows.go.
	return exec.LookPath(file)
}

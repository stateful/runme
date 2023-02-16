//go:build windows

package runner

import (
	"os"
	"os/exec"

	"github.com/pkg/errors"
)

func setSysProcAttrCtty(cmd *exec.Cmd) {}

func setSysProcAttrPgid(cmd *exec.Cmd) {}

func disableEcho(fd uintptr) error {
	return errors.New("unsupported")
}

func signalPgid(pid int, sig os.Signal) error { return errors.New("unsupported") }

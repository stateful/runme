//go:build windows

package runner

import (
	"os/exec"

	"github.com/pkg/errors"
)

func setSysProcAttrCtty(cmd *exec.Cmd) {}

func setSysProcAttrPgid(cmd *exec.Cmd) {}

func disableEcho(fd uintptr) error {
	return errors.New("unsupported")
}

func killPgid(pid int) error { return errors.New("unsupported") }

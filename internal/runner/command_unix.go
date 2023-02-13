//go:build !windows

package runner

import (
	"os/exec"
	"syscall"

	"github.com/pkg/errors"
	"github.com/pkg/term/termios"
	"golang.org/x/sys/unix"
)

func setCmdAttrs(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setsid = true
	cmd.SysProcAttr.Setctty = true
}

func disableEcho(fd uintptr) error {
	attr, err := termios.Tcgetattr(fd)
	if err != nil {
		return errors.Wrap(err, "failed to get tty attr")
	}
	attr.Lflag &^= unix.ECHO
	err = termios.Tcsetattr(fd, termios.TCSANOW, attr)
	return errors.Wrap(err, "failed to set tty attr")
}

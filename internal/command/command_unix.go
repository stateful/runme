//go:build !windows

package command

import (
	"os"
	"os/exec"
	"syscall"

	"github.com/pkg/errors"
	"github.com/pkg/term/termios"
	"golang.org/x/sys/unix"
)

func setSysProcAttrCtty(cmd *exec.Cmd, tty int) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Ctty = tty
	cmd.SysProcAttr.Setctty = true
	cmd.SysProcAttr.Setsid = true
}

func setSysProcAttrPgid(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
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

func dup(fd uintptr) (uintptr, error) {
	dupFd, err := syscall.Dup(int(fd))
	if err != nil {
		return 0, err
	}
	return uintptr(dupFd), nil
}

func closeOnExec(fd uintptr) {
	syscall.CloseOnExec(int(fd))
}

func signalPgid(pid int, sig os.Signal) error {
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		return err
	}

	s, ok := sig.(syscall.Signal)
	if !ok {
		return errors.New("os: unsupported signal type")
	}
	if e := syscall.Kill(-pgid, s); e != nil {
		if e == syscall.ESRCH {
			return os.ErrProcessDone
		}
		return e
	}
	return nil
}

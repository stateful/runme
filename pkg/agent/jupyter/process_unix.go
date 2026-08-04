//go:build !windows

package jupyter

import (
	"errors"
	"os/exec"
	"syscall"
)

func configureManagedProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func interruptManagedProcess(command *exec.Cmd) error {
	return signalManagedProcessGroup(command, syscall.SIGINT)
}

func terminateManagedProcess(command *exec.Cmd) error {
	return signalManagedProcessGroup(command, syscall.SIGTERM)
}

func killManagedProcess(command *exec.Cmd) error {
	return signalManagedProcessGroup(command, syscall.SIGKILL)
}

func signalManagedProcessGroup(command *exec.Cmd, signal syscall.Signal) error {
	if command == nil || command.Process == nil {
		return syscall.ESRCH
	}
	return syscall.Kill(-command.Process.Pid, signal)
}

func isProcessDoneError(err error) bool {
	return errors.Is(err, syscall.ESRCH)
}

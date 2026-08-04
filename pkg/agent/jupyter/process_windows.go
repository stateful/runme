//go:build windows

package jupyter

import (
	"errors"
	"os"
	"os/exec"
)

// Windows process-group control remains isolated here so it can be upgraded to
// console control events without changing KernelManager.
func configureManagedProcess(_ *exec.Cmd) {}

func interruptManagedProcess(command *exec.Cmd) error {
	if command == nil || command.Process == nil {
		return os.ErrProcessDone
	}
	return command.Process.Signal(os.Interrupt)
}

func terminateManagedProcess(command *exec.Cmd) error {
	if command == nil || command.Process == nil {
		return os.ErrProcessDone
	}
	return command.Process.Kill()
}

func killManagedProcess(command *exec.Cmd) error {
	if command == nil || command.Process == nil {
		return os.ErrProcessDone
	}
	return command.Process.Kill()
}

func isProcessDoneError(err error) bool {
	return errors.Is(err, os.ErrProcessDone)
}

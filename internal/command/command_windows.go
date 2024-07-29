//go:build windows

package command

import (
	"os"
	"os/exec"

	"github.com/pkg/errors"
)

func setSysProcAttrCtty(cmd *exec.Cmd, tty int) {}

func setSysProcAttrPgid(cmd *exec.Cmd) {}

func dup(fd uintptr) (uintptr, error) {
	return fd, nil
}

func closeOnExec(uintptr) {
	// noop
}

func disableEcho(uintptr) error {
	return errors.New("Error: Environment not supported! " +
		"Runme currently doesn't support PowerShell. " +
		"Please go to https://github.com/stateful/runme/issues/173 to follow progress on this " +
		"and join our Discord server at https://discord.gg/runme if you have further questions!")
}

func signalPgid(int, os.Signal) error { return errors.New("unsupported") }

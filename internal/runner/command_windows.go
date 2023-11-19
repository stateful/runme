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
	return errors.New("Error: Environment not supported! " +
		"Runme currently doesn't support PowerShell. " +
		"Please go to https://github.com/stateful/runme/issues/173 to follow progress on this " +
		"and join our Discord server at https://discord.gg/runme if you have further questions!")
}

func signalPgid(pid int, sig os.Signal) error { return errors.New("unsupported") }

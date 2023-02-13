//go:build windows

package runner

import (
	"os/exec"

	"github.com/pkg/errors"
)

func setCmdAttrs(cmd *exec.Cmd) {}

func disableEcho(fd uintptr) error {
	return errors.New("unsupported")
}

package server

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

func pidFileNameFromAddr(addr string) string {
	if !strings.HasPrefix(addr, "unix://") {
		return ""
	}
	path := strings.TrimPrefix(addr, "unix://")
	path = filepath.Dir(path)
	path = filepath.Join(path, "runme.pid")
	return path
}

func createFileWithPID(path string) error {
	return errors.WithStack(
		os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0o600),
	)
}

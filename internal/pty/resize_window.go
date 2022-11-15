//go:build windows
// +build windows

package pty

import "os"

func Resize(tty *os.File) CancelFn {
	return func() {}
}

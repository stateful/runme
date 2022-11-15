//go:build windows
// +build windows

package pty

import "os"

type CancelFn func()

func Resize(tty *os.File) CancelFn {
	return func() {}
}

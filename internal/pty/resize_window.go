//go:build windows
// +build windows

package pty

import "os"

type CancelFn func()

func ResizeOnSig(tty *os.File) CancelFn {
	return func() {}
}

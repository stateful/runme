//go:build !windows
// +build !windows

package pty

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/creack/pty"
)

type CancelFn func()

func ResizeOnSig(tty *os.File) CancelFn {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			if err := pty.InheritSize(os.Stdin, tty); err != nil {
				log.Printf("error resizing pty: %s", err)
			}
		}
	}()
	ch <- syscall.SIGWINCH                       // Initial resize.
	return func() { signal.Stop(ch); close(ch) } // Cleanup signals when done.
}

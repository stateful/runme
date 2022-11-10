package cmd

import (
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func shellCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:   "shell",
		Short: "Activate runme shell.",
		Long:  "Activate runme shell. This is an experimental feature.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c := exec.Command("bash", "--login", "-i")
			c.Env = append(c.Env, "RUNMESHELL=1")

			ptmx, err := pty.Start(c)
			if err != nil {
				return errors.WithMessage(err, "failed to start shell in pty")
			}
			defer func() { _ = ptmx.Close() }() // Best effort.

			ch := make(chan os.Signal, 1)
			signal.Notify(ch, syscall.SIGWINCH)
			go func() {
				for range ch {
					if err := pty.InheritSize(os.Stdin, ptmx); err != nil {
						log.Printf("error resizing pty: %s", err)
					}
				}
			}()
			ch <- syscall.SIGWINCH                        // Initial resize.
			defer func() { signal.Stop(ch); close(ch) }() // Cleanup signals when done.

			// Set stdin in raw mode.
			oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
			if err != nil {
				panic(err)
			}
			defer func() { _ = term.Restore(int(os.Stdin.Fd()), oldState) }() // Best effort.

			go func() {
				<-time.After(time.Millisecond * 500)
				_, err := ptmx.Write([]byte("echo 'wait a few seconds, it will trigger a command...'\n"))
				if err != nil {
					panic(err)
				}
			}()

			go func() {
				<-time.After(time.Second * 5)
				_, err := ptmx.Write([]byte("echo 123\n"))
				if err != nil {
					panic(err)
				}
			}()

			// Copy stdin to the pty and the pty to stdout.
			// NOTE: The goroutine will keep reading until the next keystroke before returning.
			go func() { _, _ = io.Copy(ptmx, os.Stdin) }()
			_, _ = io.Copy(os.Stdout, ptmx)

			return nil
		},
	}

	return &cmd
}

// func hijack(dst io.Writer, src io.Reader) (written int64, err error) {
// 	buf := make([]byte, 32*1024)
// 	for {
// 		nr, er := src.Read(buf)
// 		if nr > 0 {
// 			nw, ew := dst.Write(buf[0:nr])
// 			if nw < 0 || nr < nw {
// 				nw = 0
// 				if ew == nil {
// 					ew = errors.New("invalid write result")
// 				}
// 			}
// 			written += int64(nw)
// 			if ew != nil {
// 				err = ew
// 				break
// 			}
// 			if nr != nw {
// 				err = io.ErrShortWrite
// 				break
// 			}
// 		}
// 		if er != nil {
// 			if er != io.EOF {
// 				err = er
// 			}
// 			break
// 		}
// 	}
// 	return written, err
// }

package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func shellCmd() *cobra.Command {
	rand.Seed(time.Now().Unix())

	cmd := cobra.Command{
		Use:   "shell",
		Short: "Activate runme shell.",
		Long:  "Activate runme shell. This is an experimental feature.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			id := rand.Intn(1024)

			c := exec.Command("bash", "-l", "-i")
			c.Env = append(c.Env, "RUNMESHELL="+strconv.Itoa(id))

			ptmx, err := pty.Start(c)
			if err != nil {
				return errors.WithMessage(err, "failed to start shell in pty")
			}
			defer func() { _ = ptmx.Close() }()

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
				return errors.Wrap(err, "failed to put stdin in raw mode")
			}
			defer func() { _ = term.Restore(int(os.Stdin.Fd()), oldState) }()

			printf("runme: welcome to runme shell")

			// Create a temp directory to hold log files and IPC files.
			tmpdir, err := os.MkdirTemp("", "runme-"+strconv.Itoa(id)+"-*")
			if err != nil {
				return errors.Wrap(err, "failed to create a temp dir")
			}

			printf("runme: artifacts will be stored in %s", tmpdir)
			printf("") // new line

			logF, err := os.Create(filepath.Join(tmpdir, "runme.log"))
			if err != nil {
				return errors.Wrap(err, "failed to create a log file")
			}

			// When DEBUG=1 is set then duplicate all logs to stderr.
			var logW io.Writer = logF
			if debug, _ := strconv.ParseBool(os.Getenv("DEBUG")); debug {
				logW = io.MultiWriter(&terminalWriter{}, logF)
			}
			logger := zerolog.New(logW)

			// sockFile := filepath.Join(tmpdir, "runme.sock")
			sockFile := "/tmp/runme-" + strconv.Itoa(id) + ".sock"
			l, err := net.Listen("unix", sockFile)
			if err != nil {
				return errors.Wrap(err, "failed to listen to sock")
			}

			logger.Info().Stringer("addr", l.Addr()).Msg("starting to listen")

			go func() error {
				writer := singleWriter{Writer: ptmx}

				for {
					conn, err := l.Accept()
					if err != nil {
						logger.Error().Err(err).Msg("failed to accept connection")
						continue
					}

					go func() {
						for {
							buf := bufio.NewReaderSize(conn, 1024)

							data, err := buf.ReadBytes('\n')
							if err != nil {
								logger.Warn().Err(err).Msg("failed to read from a client")
								return
							}

							data = bytes.TrimSpace(data)

							go func() {
								// TODO: change it and use channels to synchronize
								time.Sleep(time.Millisecond * 300)

								w := bulkWriter{Writer: &writer}

								w.Write(data)
								w.Write([]byte("\n"))

								if _, err := w.Result(); err != nil {
									logger.Warn().Err(err).Msg("failed to write to a client")
									return
								}
							}()
						}
					}()
				}
			}()

			// Copy stdin to the pty.
			go func() {
				// TODO: copy log file
				_, err := io.Copy(ptmx, os.Stdin)
				if err != nil {
					logger.Error().Err(err).Msg("failed to copy stdin to pty")
				}
			}()

			// TODO: copy log file
			_, err = io.Copy(os.Stdout, ptmx)
			return err
		},
	}

	return &cmd
}

func printf(msg string, args ...any) {
	var buf bytes.Buffer
	_, _ = buf.WriteString("\x1b[0;32m")
	_, _ = fmt.Fprintf(&buf, msg, args...)
	_, _ = buf.WriteString("\x1b[0m")
	_, _ = buf.WriteString("\r\n")
	_, _ = os.Stderr.Write(buf.Bytes())
}

type singleWriter struct {
	io.Writer
	mu sync.Mutex
}

func (w *singleWriter) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.Writer.Write(b)
}

type terminalWriter struct{}

func (w *terminalWriter) Write(b []byte) (int, error) {
	out := os.Stderr
	n, err := os.Stderr.Write(bytes.TrimSpace(b))
	if err != nil {
		return n, err
	}
	m, err := out.Write([]byte("\r\n"))
	return n + m, err
}

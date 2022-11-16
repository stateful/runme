package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	xpty "github.com/stateful/runme/internal/pty"
	"golang.org/x/term"
)

func shellCmd() *cobra.Command {
	rand.Seed(time.Now().Unix())

	var commandName string

	cmd := cobra.Command{
		Use:   "shell",
		Short: "Activate runme shell.",
		Long:  "Activate runme shell. This is an experimental feature.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			id := rand.Intn(1024)

			c := exec.Command(commandName)
			c.Env = append(os.Environ(), "RUNMESHELL="+strconv.Itoa(id))

			ptmx, err := pty.Start(c)
			if err != nil {
				return errors.WithMessage(err, "failed to start shell in pty")
			}
			defer func() { _ = ptmx.Close() }()

			cancelResize := xpty.ResizeOnSig(ptmx)
			defer cancelResize()

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
			logger := zerolog.New(logF)

			sockPath := "/tmp/runme-" + strconv.Itoa(id) + ".sock"
			var lc net.ListenConfig
			l, err := lc.Listen(cmd.Context(), "unix", sockPath)
			if err != nil {
				return errors.Wrap(err, "failed to listen to sock")
			}
			defer func() { _ = l.Close(); _ = os.Remove(sockPath) }()

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
								// TODO: use channels to synchronize instead of sleeping.
								// I don't have a better idea than using ptrace to
								// monitor the state of the shell. If there are no child processes,
								// then it means that the shell is not busy and can be written to.
								//
								// Alternatively, eBPF might be also a good idea.
								time.Sleep(time.Second)

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

	defaultShell := os.Getenv("SHELL")
	if defaultShell == "" {
		defaultShell, _ = exec.LookPath("bash")
		if defaultShell == "" {
			defaultShell = "/bin/sh"
		}
	}

	cmd.Flags().StringVar(&commandName, "command", defaultShell, "Command to execute and watch.")

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

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
	"strings"
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
			// detect promptDetectOut
			var (
				promptDetectOut []byte
				err             error
			)
			switch {
			case strings.HasSuffix(commandName, "bash"):
				promptDetectOut, err = exec.Command(commandName, "-i", "-c", "echo ${PS1@P}").CombinedOutput()
			case strings.HasSuffix(commandName, "zsh"):
				promptDetectOut, err = exec.Command(commandName, "-i", "-c", "print -P $PS1").CombinedOutput()
			default:
				err = errors.New("unsupported shell")
			}
			if len(promptDetectOut) == 0 {
				err = errors.New("empty prompt")
			}
			if err != nil {
				return errors.Wrapf(err, "failed to detect a valid prompt: %s", promptDetectOut)
			}

			promptSlice := bytes.Split(promptDetectOut, []byte{'\n'})

			// Find the last non-empty line and consider this to be a prompt
			// we will be looking for.
			var prompt []byte
			for i := len(promptSlice) - 1; i >= 0; i-- {
				s := promptSlice[i]
				if len(s) > 0 {
					prompt = s
					break
				}
			}
			printf("detected prompt: %s", prompt)

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

			// Logger setup
			zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

			logF, err := os.Create(filepath.Join(tmpdir, "runme.log"))
			if err != nil {
				return errors.Wrap(err, "failed to create a log file")
			}

			logger := zerolog.New(logF)

			sockPath := "/tmp/runme-" + strconv.Itoa(id) + ".sock"
			var lc net.ListenConfig
			// TODO: context should be closed only after the goroutine below is closed.
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
					// TODO: handle the case when a conn is closed
					if err != nil {
						logger.Error().Err(err).Msg("failed to accept connection")
						continue
					}

					go func() {
						for {
							r := bufio.NewReader(conn)
							data, err := r.ReadBytes('\n')
							if err != nil {
								logger.Warn().Err(err).Msg("failed to read from a client")
								return
							}
							data = bytes.TrimSpace(data)

							if len(data) == 0 {
								logger.Info().Msg("read empty line from a client")
								return
							}

							logger.Info().Str("data", string(data)).Msg("read from client")

							go func() {
								// TODO: use detected prompt to figure this out.
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

			go func() {
				// TODO: copy log file
				_, err := io.Copy(ptmx, os.Stdin)
				if err != nil {
					logger.Error().Err(err).Msg("failed to copy stdin to pty")
				}
			}()

			_, err = io.Copy(os.Stdout, ptmx)
			return err
		},
	}

	defaultShell := os.Getenv("SHELL")
	if defaultShell == "" {
		defaultShell, _ = exec.LookPath("bash")
	}
	if defaultShell == "" {
		defaultShell = "/bin/sh"
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

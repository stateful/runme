package runnerv2service

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"regexp"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/gabriel-vasile/mimetype"
	"github.com/pkg/errors"

	"github.com/stateful/runme/v3/internal/command"
	"github.com/stateful/runme/v3/internal/rbuffer"
	"github.com/stateful/runme/v3/internal/session"
	runnerv2 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/runner/v2"
	"github.com/stateful/runme/v3/pkg/project"
)

var opininatedEnvVarNamingRegexp = regexp.MustCompile(`^[A-Z_][A-Z0-9_]{1}[A-Z0-9_]*[A-Z][A-Z0-9_]*$`)

func matchesOpinionatedEnvVarNaming(knownName string) bool {
	return opininatedEnvVarNamingRegexp.MatchString(knownName)
}

type execution struct {
	Cmd command.Command

	knownName        string
	logger           *zap.Logger
	session          *session.Session
	storeStdoutInEnv bool

	stdinR, stdoutR, stderrR io.Reader
	stdinW, stdoutW, stderrW io.WriteCloser
}

func newExecution(
	cfg *command.ProgramConfig,
	proj *project.Project,
	session *session.Session,
	logger *zap.Logger,
	storeStdoutInEnv bool,
) (*execution, error) {
	logger = logger.Named("execution")

	cmdFactory := command.NewFactory(
		command.WithProject(proj),
		command.WithLogger(logger),
	)

	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()
	stderrR, stderrW := io.Pipe()

	cmdOptions := command.CommandOptions{
		EnableEcho:  true,
		Session:     session,
		StdinWriter: stdinW,
		Stdin:       stdinR,
		Stdout:      stdoutW,
		Stderr:      stderrW,
	}

	cmd, err := cmdFactory.Build(cfg, cmdOptions)
	if err != nil {
		return nil, err
	}

	exec := &execution{
		Cmd: cmd,

		knownName:        cfg.GetKnownName(),
		logger:           logger,
		session:          session,
		storeStdoutInEnv: storeStdoutInEnv,

		stdinR:  stdinR,
		stdinW:  stdinW,
		stdoutR: stdoutR,
		stdoutW: stdoutW,
		stderrR: stderrR,
		stderrW: stderrW,
	}
	return exec, nil
}

func (e *execution) closeIO() {
	err := e.stdinW.Close()
	e.logger.Info("closed stdin writer", zap.Error(err))

	err = e.stdoutW.Close()
	e.logger.Info("closed stdout writer", zap.Error(err))

	err = e.stderrW.Close()
	e.logger.Info("closed stderr writer", zap.Error(err))
}

func (e *execution) storeOutputInEnv(ctx context.Context, r io.Reader) {
	b, err := io.ReadAll(r)
	if err != nil {
		e.logger.Warn("failed to read last output", zap.Error(err))
		return
	}

	sanitized := bytes.ReplaceAll(b, []byte{'\000'}, nil)
	env := command.CreateEnv(command.StoreStdoutEnvName, string(sanitized))
	if err := e.session.SetEnv(ctx, env); err != nil {
		e.logger.Warn("failed to store last output", zap.Error(err))
	}

	if e.knownName != "" && matchesOpinionatedEnvVarNaming(e.knownName) {
		if err := e.session.SetEnv(ctx, e.knownName+"="+string(sanitized)); err != nil {
			e.logger.Warn("failed to store output under known name", zap.String("known_name", e.knownName), zap.Error(err))
		}
	}
}

func (e *execution) Wait(ctx context.Context, sender runnerv2.RunnerService_ExecuteServer) (int, error) {
	envStdout := io.Discard
	if e.storeStdoutInEnv {
		b := rbuffer.NewRingBuffer(session.MaxEnvSizeInBytes - len(command.StoreStdoutEnvName) - 1)
		defer func() {
			_ = b.Close()
			e.storeOutputInEnv(ctx, b)
		}()
		envStdout = b
	}

	readSendDone := make(chan error, 2)
	go func() {
		mimetypeDetected := false

		readSendDone <- e.readSendLoop(
			sender,
			e.stdoutR,
			func(b []byte) *runnerv2.ExecuteResponse {
				if _, err := envStdout.Write(b); err != nil {
					e.logger.Warn("failed to write to envStdout writer", zap.Error(err))
					envStdout = io.Discard
				}

				response := &runnerv2.ExecuteResponse{
					StdoutData: b,
				}

				if !mimetypeDetected {
					if detected := mimetype.Detect(response.StdoutData); detected != nil {
						mimetypeDetected = true
						response.MimeType = detected.String()
						e.logger.Debug("detected MIME type", zap.String("mime", detected.String()))
					} else {
						e.logger.Debug("failed to detect MIME type")
					}
				}

				return response
			},
			e.logger.Named("readSendLoop.stdout"),
		)
	}()
	go func() {
		readSendDone <- e.readSendLoop(
			sender,
			e.stderrR,
			func(b []byte) *runnerv2.ExecuteResponse {
				return &runnerv2.ExecuteResponse{
					StderrData: b,
				}
			},
			e.logger.Named("readSendLoop.stderr"),
		)
	}()

	waitErr := e.Cmd.Wait(ctx)
	exitCode := exitCodeFromErr(waitErr)
	e.logger.Info("command finished", zap.Int("exitCode", exitCode), zap.Error(waitErr))

	e.closeIO()

	if waitErr != nil {
		// Drain the readSendDone channel to avoid goroutine leaks.
		for i := 0; i < cap(readSendDone); i++ {
			<-readSendDone
		}
		return exitCode, waitErr
	}

	readSendLoopsFinished := 0

finalWait:
	select {
	case <-ctx.Done():
		e.logger.Info("context done", zap.Error(ctx.Err()))
		// Drain the readSendDone channel to avoid goroutine leaks.
		for i := 0; i < cap(readSendDone); i++ {
			<-readSendDone
		}
		return exitCode, ctx.Err()
	case err := <-readSendDone:
		if err != nil {
			e.logger.Info("readSendCtx done", zap.Error(err))
		}
		readSendLoopsFinished++
		if readSendLoopsFinished < cap(readSendDone) {
			goto finalWait
		}
		return exitCode, err
	}
}

func (e *execution) readSendLoop(
	sender runnerv2.RunnerService_ExecuteServer,
	src io.Reader,
	cb func([]byte) *runnerv2.ExecuteResponse,
	logger *zap.Logger,
) error {
	// Limit to 30 sends per second. This is typically quite enough
	// for interactive commands and streaming the output.
	const sendsPerSecond = 30

	// This is a thread-safe buffer.
	// Data from `src` is copied to this buffer
	// in a goroutine and then read from it
	// in the main loop.
	buf := newBuffer(msgBufferSize)

	// Copy from src to buffer.
	go func() {
		n, err := io.Copy(buf, src)
		logger.Debug("copied from source to buffer", zap.Int64("count", n), zap.Error(err))
		_ = buf.Close() // always nil
	}()

	data := make([]byte, msgBufferSize)

	for {
		eof := false
		n, err := buf.Read(data)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				return errors.WithStack(err)
			}
			eof = true
		}
		logger.Debug("read", zap.Int("n", n), zap.Bool("eof", eof))
		if n == 0 {
			if eof {
				return nil
			}
			continue
		}

		readTime := time.Now()

		response := cb(data[:n])
		if err := sender.Send(response); err != nil {
			return errors.WithStack(err)
		}

		if n < msgBufferSize {
			time.Sleep(time.Second/sendsPerSecond - time.Since(readTime))
		}
	}
}

func (e *execution) Write(p []byte) (int, error) {
	n, err := e.stdinW.Write(p)

	e.logger.Debug("wrote to stdin", zap.Int("payload", len(p)), zap.Int("n", n), zap.Error(err))

	// Close stdin writer for non-interactive commands after handling the initial request.
	// Non-interactive commands do not support sending data continuously and require that
	// the stdin writer to be closed to finish processing the input.
	if ok := e.Cmd.Interactive(); !ok {
		if closeErr := e.stdinW.Close(); closeErr != nil {
			e.logger.Info("failed to close native command stdin writer", zap.Error(closeErr))
			if err == nil {
				err = closeErr
			}
		}
	}

	return n, errors.WithStack(err)
}

func (e *execution) SetWinsize(size *runnerv2.Winsize) error {
	if size == nil {
		return nil
	}

	return command.SetWinsize(
		e.Cmd,
		&command.Winsize{
			Rows: uint16(size.Rows),
			Cols: uint16(size.Cols),
			X:    uint16(size.X),
			Y:    uint16(size.Y),
		},
	)
}

func (e *execution) Stop(stop runnerv2.ExecuteStop) (err error) {
	e.logger.Info("stopping program", zap.Any("stop", stop))

	switch stop {
	case runnerv2.ExecuteStop_EXECUTE_STOP_UNSPECIFIED:
		// continue
	case runnerv2.ExecuteStop_EXECUTE_STOP_INTERRUPT:
		err = e.Cmd.Signal(os.Interrupt)
	case runnerv2.ExecuteStop_EXECUTE_STOP_KILL:
		err = e.Cmd.Signal(os.Kill)
	default:
		err = errors.New("unknown stop signal")
	}
	return
}

func exitCodeFromErr(err error) int {
	if err == nil {
		return 0
	}
	var exiterr *exec.ExitError
	if errors.As(err, &exiterr) {
		status, ok := exiterr.ProcessState.Sys().(syscall.WaitStatus)
		if ok && status.Signaled() {
			// TODO(adamb): will like need to be improved.
			if status.Signal() == os.Interrupt {
				return 130
			} else if status.Signal() == os.Kill {
				return 137
			}
		}
		return exiterr.ExitCode()
	}
	return -1
}

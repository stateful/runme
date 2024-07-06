package runnerv2service

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"regexp"
	"syscall"

	"github.com/gabriel-vasile/mimetype"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/stateful/runme/v3/internal/command"
	"github.com/stateful/runme/v3/internal/rbuffer"
	runnerv2alpha1 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/runner/v2alpha1"
	"github.com/stateful/runme/v3/pkg/project"
)

const (
	// ringBufferSize limits the size of the ring buffers
	// that sit between a command and the handler.
	ringBufferSize = 8192 << 10 // 8 MiB

	// msgBufferSize limits the size of data chunks
	// sent by the handler to clients. It's smaller
	// intentionally as typically the messages are
	// small.
	// In the future, it might be worth to implement
	// variable-sized buffers.
	msgBufferSize = 2048 << 10 // 2 MiB
)

// Only allow uppercase letters, digits and underscores, min three chars
var OpininatedEnvVarNamingRegexp = regexp.MustCompile(`^[A-Z_][A-Z0-9_]{1}[A-Z0-9_]*[A-Z][A-Z0-9_]*$`)

type execution struct {
	ID        string
	KnownName string
	Cmd       command.Command

	session          *command.Session
	storeStdoutInEnv bool

	stdin       io.Reader
	stdinWriter io.WriteCloser
	stdout      *rbuffer.RingBuffer
	stderr      *rbuffer.RingBuffer

	logger *zap.Logger
}

func newExecution(
	id string,
	storeStdoutInEnv bool,
	cfg *command.ProgramConfig,
	proj *project.Project,
	session *command.Session,
	logger *zap.Logger,
) *execution {
	cmdFactory := command.NewFactory(
		command.WithLogger(logger),
		command.WithProject(proj),
	)

	stdin, stdinWriter := io.Pipe()
	stdout := rbuffer.NewRingBuffer(ringBufferSize)
	stderr := rbuffer.NewRingBuffer(ringBufferSize)

	cmdOptions := command.CommandOptions{
		EnableEcho:  true,
		Session:     session,
		StdinWriter: stdinWriter,
		Stdin:       stdin,
		Stdout:      stdout,
		Stderr:      stderr,
	}

	cmd := cmdFactory.Build(cfg, cmdOptions)

	exec := &execution{
		ID:        id,
		KnownName: cfg.GetKnownName(),
		Cmd:       cmd,

		session:          session,
		storeStdoutInEnv: storeStdoutInEnv,

		stdin:       stdin,
		stdinWriter: stdinWriter,
		stdout:      stdout,
		stderr:      stderr,

		logger: logger,
	}

	return exec
}

func (e *execution) Wait(ctx context.Context, sender sender) (int, error) {
	lastStdout := rbuffer.NewRingBuffer(command.MaxEnvironSizeInBytes)
	defer func() {
		_ = lastStdout.Close()
		e.storeOutputInEnv(lastStdout)
	}()

	firstStdoutSent := false
	errc := make(chan error, 2)

	go func() {
		errc <- readSendLoop(
			e.stdout,
			sender,
			func(b []byte) *runnerv2alpha1.ExecuteResponse {
				if len(b) == 0 {
					return nil
				}

				_, _ = lastStdout.Write(b)

				resp := &runnerv2alpha1.ExecuteResponse{StdoutData: b}

				if !firstStdoutSent {
					if detected := mimetype.Detect(b); detected != nil {
						e.logger.Info("detected MIME type", zap.String("mime", detected.String()))
						resp.MimeType = detected.String()
					}
				}

				firstStdoutSent = true

				e.logger.Debug("sending stdout data", zap.Any("resp", resp))

				return resp
			},
		)
	}()
	go func() {
		errc <- readSendLoop(
			e.stderr,
			sender,
			func(b []byte) *runnerv2alpha1.ExecuteResponse {
				if len(b) == 0 {
					return nil
				}
				resp := &runnerv2alpha1.ExecuteResponse{StderrData: b}
				e.logger.Debug("sending stderr data", zap.Any("resp", resp))
				return resp
			},
		)
	}()

	waitErr := e.Cmd.Wait()
	exitCode := exitCodeFromErr(waitErr)

	e.closeIO()

	// If waitErr is not nil, only log the errors but return waitErr.
	if waitErr != nil {
		handlerErrors := 0

	readSendHandlerForWaitErr:
		select {
		case err := <-errc:
			handlerErrors++
			e.logger.Info("readSendLoop finished; ignoring any errors because there was a wait error", zap.Error(err))
			// Wait for both errors, or nils.
			if handlerErrors < 2 {
				goto readSendHandlerForWaitErr
			}
		case <-ctx.Done():
			e.logger.Info("context canceled while waiting for the readSendLoop finish; ignoring any errors because there was a wait error")
		}
		return exitCode, waitErr
	}

	// If waitErr is nil, wait for the readSendLoop to finish,
	// or the context being canceled.
	select {
	case err1 := <-errc:
		// Wait for both errors, or nils.
		select {
		case err2 := <-errc:
			if err2 != nil {
				e.logger.Info("another error from readSendLoop; won't be returned", zap.Error(err2))
			}
		case <-ctx.Done():
		}
		return exitCode, err1
	case <-ctx.Done():
		return exitCode, ctx.Err()
	}
}

func (e *execution) Write(p []byte) (int, error) {
	n, err := e.stdinWriter.Write(p)

	// Close stdin writer for non-interactive commands after handling the initial request.
	// Non-interactive commands do not support sending data continuously and require that
	// the stdin writer to be closed to finish processing the input.
	if ok := e.Cmd.Interactive(); !ok {
		if closeErr := e.stdinWriter.Close(); closeErr != nil {
			e.logger.Info("failed to close native command stdin writer", zap.Error(closeErr))
			if err == nil {
				err = closeErr
			}
		}
	}

	return n, errors.WithStack(err)
}

func (e *execution) SetWinsize(size *runnerv2alpha1.Winsize) error {
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

func (e *execution) Stop(stop runnerv2alpha1.ExecuteStop) (err error) {
	switch stop {
	case runnerv2alpha1.ExecuteStop_EXECUTE_STOP_UNSPECIFIED:
		// continue
	case runnerv2alpha1.ExecuteStop_EXECUTE_STOP_INTERRUPT:
		err = e.Cmd.Signal(os.Interrupt)
	case runnerv2alpha1.ExecuteStop_EXECUTE_STOP_KILL:
		err = e.Cmd.Signal(os.Kill)
	default:
		err = errors.New("unknown stop signal")
	}
	return
}

func (e *execution) closeIO() {
	err := e.stdinWriter.Close()
	e.logger.Info("closed stdin writer", zap.Error(err))

	err = e.stdout.Close()
	e.logger.Info("closed stdout writer", zap.Error(err))

	err = e.stderr.Close()
	e.logger.Info("closed stderr writer", zap.Error(err))
}

func (e *execution) storeOutputInEnv(r io.Reader) {
	if !e.storeStdoutInEnv {
		return
	}

	b, _ := io.ReadAll(r)
	sanitized := bytes.ReplaceAll(b, []byte{'\000'}, nil)

	if err := e.session.SetEnv("__=" + string(sanitized)); err != nil {
		e.logger.Info("failed to store last output", zap.Error(err))
	}

	if e.KnownName != "" && conformsOpinionatedEnvVarNaming(e.KnownName) {
		if err := e.session.SetEnv(e.KnownName + "=" + string(sanitized)); err != nil {
			e.logger.Info("failed to store output under known name", zap.String("known_name", e.KnownName), zap.Error(err))
		}
	}
}

func conformsOpinionatedEnvVarNaming(knownName string) bool {
	return OpininatedEnvVarNamingRegexp.MatchString(knownName)
}

type sender interface {
	Send(*runnerv2alpha1.ExecuteResponse) error
}

func readSendLoop(reader io.Reader, sender sender, fn func([]byte) *runnerv2alpha1.ExecuteResponse) error {
	limitedReader := io.LimitReader(reader, msgBufferSize)

	for {
		buf := make([]byte, msgBufferSize)
		n, err := limitedReader.Read(buf)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return errors.WithStack(err)
		}
		if n == 0 {
			continue
		}

		msg := fn(buf[:n])
		if msg == nil {
			return nil
		}
		err = sender.Send(msg)
		if err != nil {
			return errors.WithStack(err)
		}
	}
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

package runnerv2service

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/gabriel-vasile/mimetype"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/stateful/runme/v3/internal/command"
	"github.com/stateful/runme/v3/internal/rbuffer"
	runnerv2 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/runner/v2"
	"github.com/stateful/runme/v3/pkg/project"
)

const (
	// msgBufferSize limits the size of data chunks
	// sent by the handler to clients. It's smaller
	// intentionally as typically the messages are
	// small.
	// In the future, it might be worth to implement
	// variable-sized buffers.
	msgBufferSize = 2 * 1024 * 1024 // 2 MiB
)

var opininatedEnvVarNamingRegexp = regexp.MustCompile(`^[A-Z_][A-Z0-9_]{1}[A-Z0-9_]*[A-Z][A-Z0-9_]*$`)

type buffer struct {
	mu     *sync.Mutex
	b      *bytes.Buffer
	closed *atomic.Bool
	close  chan struct{}
	more   chan struct{}
}

var _ io.WriteCloser = (*buffer)(nil)

func newBuffer() *buffer {
	return &buffer{
		mu:     &sync.Mutex{},
		b:      bytes.NewBuffer(make([]byte, 0, msgBufferSize)),
		closed: &atomic.Bool{},
		close:  make(chan struct{}),
		more:   make(chan struct{}),
	}
}

func (b *buffer) Write(p []byte) (int, error) {
	if b.closed.Load() {
		return 0, errors.New("closed")
	}

	b.mu.Lock()
	n, err := b.b.Write(p)
	b.mu.Unlock()

	select {
	case b.more <- struct{}{}:
	default:
	}

	return n, err
}

func (b *buffer) Close() error {
	if b.closed.CompareAndSwap(false, true) {
		close(b.close)
	}
	return nil
}

func (b *buffer) Read(p []byte) (int, error) {
	b.mu.Lock()
	n, err := b.b.Read(p)
	b.mu.Unlock()

	if err != nil && errors.Is(err, io.EOF) && !b.closed.Load() {
		select {
		case <-b.more:
		case <-b.close:
			return n, io.EOF
		}
		return n, nil
	}

	return n, err
}

type execution struct {
	Cmd              command.Command
	knownName        string
	logger           *zap.Logger
	session          *command.Session
	stdin            io.Reader
	stdinWriter      io.WriteCloser
	stdout           *buffer
	stderr           *buffer
	storeStdoutInEnv bool
}

func newExecution(
	cfg *command.ProgramConfig,
	proj *project.Project,
	session *command.Session,
	logger *zap.Logger,
	storeStdoutInEnv bool,
) (*execution, error) {
	cmdFactory := command.NewFactory(
		command.WithLogger(logger),
		command.WithProject(proj),
	)

	stdin, stdinWriter := io.Pipe()
	stdout := newBuffer()
	stderr := newBuffer()

	cmdOptions := command.CommandOptions{
		EnableEcho:  true,
		Session:     session,
		StdinWriter: stdinWriter,
		Stdin:       stdin,
		Stdout:      stdout,
		Stderr:      stderr,
	}

	cmd, err := cmdFactory.Build(cfg, cmdOptions)
	if err != nil {
		return nil, err
	}

	exec := &execution{
		Cmd:              cmd,
		knownName:        cfg.GetKnownName(),
		logger:           logger,
		session:          session,
		stdin:            stdin,
		stdinWriter:      stdinWriter,
		stdout:           stdout,
		stderr:           stderr,
		storeStdoutInEnv: storeStdoutInEnv,
	}

	return exec, nil
}

func (e *execution) Wait(ctx context.Context, sender sender) (int, error) {
	lastStdout := io.Discard
	if e.storeStdoutInEnv {
		b := rbuffer.NewRingBuffer(command.MaxEnvSizeInBytes - len(command.StoreStdoutEnvName) - 1)
		defer func() {
			_ = b.Close()
			e.storeOutputInEnv(b)
		}()
		lastStdout = b
	}

	firstStdoutSent := false
	errc := make(chan error, 2)

	go func() {
		errc <- readSendLoop(
			e.stdout,
			sender,
			func(b []byte) *runnerv2.ExecuteResponse {
				if len(b) == 0 {
					return nil
				}

				_, err := lastStdout.Write(b)
				if err != nil {
					e.logger.Warn("failed to write last output", zap.Error(err))
				}

				resp := &runnerv2.ExecuteResponse{StdoutData: b}

				if !firstStdoutSent {
					if detected := mimetype.Detect(b); detected != nil {
						e.logger.Info("detected MIME type", zap.String("mime", detected.String()))
						resp.MimeType = detected.String()
					}
				}
				firstStdoutSent = true

				e.logger.Debug("sending stdout data", zap.Int("len", len(resp.StdoutData)))

				return resp
			},
			e.logger.With(zap.String("source", "stdout")),
		)
	}()
	go func() {
		errc <- readSendLoop(
			e.stderr,
			sender,
			func(b []byte) *runnerv2.ExecuteResponse {
				if len(b) == 0 {
					return nil
				}
				resp := &runnerv2.ExecuteResponse{StderrData: b}
				e.logger.Debug("sending stderr data", zap.Any("resp", resp))
				return resp
			},
			e.logger.With(zap.String("source", "stderr")),
		)
	}()

	waitErr := e.Cmd.Wait()
	exitCode := exitCodeFromErr(waitErr)

	e.logger.Info("command finished", zap.Int("exitCode", exitCode), zap.Error(waitErr))

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

func (e *execution) closeIO() {
	err := e.stdinWriter.Close()
	e.logger.Info("closed stdin writer", zap.Error(err))

	err = e.stdout.Close()
	e.logger.Info("closed stdout writer", zap.Error(err))

	err = e.stderr.Close()
	e.logger.Info("closed stderr writer", zap.Error(err))
}

func (e *execution) storeOutputInEnv(r io.Reader) {
	b, err := io.ReadAll(r)
	if err != nil {
		e.logger.Warn("failed to read last output", zap.Error(err))
		return
	}

	sanitized := bytes.ReplaceAll(b, []byte{'\000'}, nil)
	env := command.CreateEnv(command.StoreStdoutEnvName, string(sanitized))
	if err := e.session.SetEnv(env); err != nil {
		e.logger.Warn("failed to store last output", zap.Error(err))
	}

	if e.knownName != "" && matchesOpinionatedEnvVarNaming(e.knownName) {
		if err := e.session.SetEnv(e.knownName + "=" + string(sanitized)); err != nil {
			e.logger.Warn("failed to store output under known name", zap.String("known_name", e.knownName), zap.Error(err))
		}
	}
}

func matchesOpinionatedEnvVarNaming(knownName string) bool {
	return opininatedEnvVarNamingRegexp.MatchString(knownName)
}

type sender interface {
	Send(*runnerv2.ExecuteResponse) error
}

func readSendLoop(
	reader io.Reader,
	sender sender,
	fn func([]byte) *runnerv2.ExecuteResponse,
	logger *zap.Logger,
) error {
	buf := make([]byte, msgBufferSize)

	for {
		eof := false
		n, err := reader.Read(buf)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				return errors.WithStack(err)
			}
			eof = true
		}

		logger.Info("readSendLoop", zap.Int("n", n))

		if n == 0 && eof {
			return nil
		}

		msg := fn(buf[:n])
		if msg == nil {
			continue
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

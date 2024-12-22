package runnerv2service

import (
	"bytes"
	"context"
	"io"
	"os"
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

const (
	// msgBufferSize limits the size of data chunks
	// sent by the handler to clients. It's smaller
	// intentionally as typically the messages are
	// small.
	// In the future, it might be worth to implement
	// variable-sized buffers.
	msgBufferSize = 32 * 1024 * 1024 // 4 MiB
)

//lint:ignore U1000 Used in A/B testing
type execution2 struct {
	Cmd command.Command

	knownName        string
	logger           *zap.Logger
	session          *session.Session
	storeStdoutInEnv bool

	stdinR, stdoutR, stderrR io.Reader
	stdinW, stdoutW, stderrW io.WriteCloser
}

//lint:ignore U1000 Used in A/B testing
func newExecution2(
	cfg *command.ProgramConfig,
	proj *project.Project,
	session *session.Session,
	logger *zap.Logger,
	storeStdoutInEnv bool,
) (*execution2, error) {
	logger = logger.Named("execution2")

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

	exec := &execution2{
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

func (e *execution2) closeIO() {
	err := e.stdinW.Close()
	e.logger.Info("closed stdin writer", zap.Error(err))

	err = e.stdoutW.Close()
	e.logger.Info("closed stdout writer", zap.Error(err))

	err = e.stderrW.Close()
	e.logger.Info("closed stderr writer", zap.Error(err))
}

func (e *execution2) storeOutputInEnv(ctx context.Context, r io.Reader) {
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

func (e *execution2) Wait(ctx context.Context, sender runnerv2.RunnerService_ExecuteServer) (int, error) {
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
		return exitCode, waitErr
	}

	readSendLoopsFinished := 0

finalWait:
	select {
	case <-ctx.Done():
		e.logger.Info("context done", zap.Error(ctx.Err()))
		return exitCode, ctx.Err()
	case err := <-readSendDone:
		if err != nil {
			e.logger.Info("readSendCtx done", zap.Error(err))
		}
		readSendLoopsFinished++
		if readSendLoopsFinished < 2 {
			goto finalWait
		}
		return exitCode, err
	}
}

func (e *execution2) readSendLoop(
	sender runnerv2.RunnerService_ExecuteServer,
	src io.Reader,
	cb func([]byte) *runnerv2.ExecuteResponse,
	logger *zap.Logger,
) error {
	const sendsPerSecond = 30

	buf := newBuffer(msgBufferSize)

	// Copy from src to [buffer].
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

func (e *execution2) Write(p []byte) (int, error) {
	n, err := e.stdinW.Write(p)

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

func (e *execution2) SetWinsize(size *runnerv2.Winsize) error {
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

func (e *execution2) Stop(stop runnerv2.ExecuteStop) (err error) {
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

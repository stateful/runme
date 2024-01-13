package runnerv2service

import (
	"context"
	"io"
	"os"
	"os/exec"
	"syscall"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/stateful/runme/internal/command"
	runnerv2alpha1 "github.com/stateful/runme/internal/gen/proto/go/runme/runner/v2alpha1"
	"github.com/stateful/runme/internal/rbuffer"
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

type execution struct {
	ID string

	Cmd *command.VirtualCommand

	stdin       io.Reader
	stdinWriter io.WriteCloser
	stdout      *rbuffer.RingBuffer

	logger *zap.Logger
}

func newExecution(id string, cfg *command.Config, logger *zap.Logger) (*execution, error) {
	var (
		stdin       io.Reader
		stdinWriter io.WriteCloser
	)

	if cfg.Interactive {
		stdin, stdinWriter = io.Pipe()
	}

	stdout := rbuffer.NewRingBuffer(ringBufferSize)

	cmd, err := command.NewVirtual(
		cfg,
		&command.VirtualCommandOptions{
			Stdin:  stdin,
			Stdout: stdout,
			Logger: logger,
		},
	)
	if err != nil {
		return nil, err
	}

	exec := &execution{
		ID:  id,
		Cmd: cmd,

		stdin:       stdin,
		stdinWriter: stdinWriter,
		stdout:      stdout,

		logger: logger,
	}

	return exec, nil
}

func (e *execution) Start(ctx context.Context) error {
	return e.Cmd.Start(ctx)
}

func (e *execution) Wait(ctx context.Context, sender sender) (int, error) {
	errc := make(chan error, 1)
	go func() {
		errc <- readSendLoop(e.stdout, sender)
	}()

	waitErr := e.Cmd.Wait()
	exitCode := exitCodeFromErr(waitErr)

	e.closeIO()

	// If waitErr is not nil, only log the errors but return waitErr.
	if waitErr != nil {
		select {
		case err := <-errc:
			e.logger.Info("readSendLoop finished; ignoring any errors because there was a wait error", zap.Error(err))
		case <-ctx.Done():
			e.logger.Info("context canceled while waiting for the readSendLoop finish; ignoring any errors because there was a wait error")
		}
		return exitCode, waitErr
	}

	// If waitErr is nil, wait for the readSendLoop to finish,
	// or the context being canceled.
	select {
	case err := <-errc:
		return exitCode, err
	case <-ctx.Done():
		return exitCode, ctx.Err()
	}
}

func (e *execution) Write(p []byte) (int, error) {
	n, err := e.stdinWriter.Write(p)
	return n, errors.WithStack(err)
}

func (e *execution) SetWinsize(size *runnerv2alpha1.Winsize) error {
	return e.Cmd.SetWinsize(uint16(size.Cols), uint16(size.Rows), uint16(size.X), uint16(size.Y))
}

func (e *execution) closeIO() {
	var err error

	if e.stdinWriter != nil {
		err = e.stdinWriter.Close()
		e.logger.Debug("closed stdin writer", zap.Error(err))
	}

	err = e.stdout.Close()
	e.logger.Debug("closed stdout writer", zap.Error(err))
}

type sender interface {
	Send(*runnerv2alpha1.ExecuteResponse) error
}

func readSendLoop(reader io.Reader, sender sender) error {
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

		err = sender.Send(&runnerv2alpha1.ExecuteResponse{StdoutData: buf[:n]})
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

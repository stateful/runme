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

type commandIface interface {
	Pid() int
	Running() bool
	SetWinsize(uint16, uint16, uint16, uint16) error
	Start(context.Context) error
	StopWithSignal(os.Signal) error
	Wait() error
}

type execution struct {
	ID string

	Cmd commandIface

	stdin       io.Reader
	stdinWriter io.WriteCloser
	stdout      *rbuffer.RingBuffer
	stderr      *rbuffer.RingBuffer

	logger *zap.Logger
}

func newExecution(id string, cfg *command.Config, logger *zap.Logger) (*execution, error) {
	stdin, stdinWriter := io.Pipe()
	stdout := rbuffer.NewRingBuffer(ringBufferSize)
	stderr := rbuffer.NewRingBuffer(ringBufferSize)

	var (
		cmd commandIface
		err error
	)

	if cfg.Interactive {
		cmd, err = command.NewVirtual(
			cfg,
			&command.VirtualCommandOptions{
				Stdin:  stdin,
				Stdout: stdout,
				Logger: logger,
			},
		)
	} else {
		cmd, err = command.NewNative(
			cfg,
			&command.NativeCommandOptions{
				Stdin:  stdin,
				Stdout: stdout,
				Stderr: stderr,
				Logger: logger,
			},
		)
	}

	if err != nil {
		return nil, err
	}

	exec := &execution{
		ID:  id,
		Cmd: cmd,

		stdin:       stdin,
		stdinWriter: stdinWriter,
		stdout:      stdout,
		stderr:      stderr,

		logger: logger,
	}

	return exec, nil
}

func (e *execution) Start(ctx context.Context) error {
	return e.Cmd.Start(ctx)
}

func (e *execution) Wait(ctx context.Context, sender sender) (int, error) {
	errc := make(chan error, 2)

	go func() {
		errc <- readSendLoop(e.stdout, sender, func(b []byte) *runnerv2alpha1.ExecuteResponse { return &runnerv2alpha1.ExecuteResponse{StdoutData: b} })
	}()
	go func() {
		errc <- readSendLoop(e.stderr, sender, func(b []byte) *runnerv2alpha1.ExecuteResponse { return &runnerv2alpha1.ExecuteResponse{StderrData: b} })
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
			e.logger.Info("another error from readSendLoop; won't be returned", zap.Error(err2))
		case <-ctx.Done():
		}
		return exitCode, err1
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

func (e *execution) PostInitialRequest() {
	// Close stdin writer for native commands after handling the initial request.
	// Native commands do not support sending data continously.
	if _, ok := e.Cmd.(*command.NativeCommand); ok {
		if err := e.stdinWriter.Close(); err != nil {
			e.logger.Info("failed to close stdin writer", zap.Error(err))
		}
	}
}

func (e *execution) closeIO() {
	err := e.stdinWriter.Close()
	e.logger.Info("closed stdin writer", zap.Error(err))

	err = e.stdout.Close()
	e.logger.Info("closed stdout writer", zap.Error(err))

	err = e.stderr.Close()
	e.logger.Info("closed stderr writer", zap.Error(err))
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

		err = sender.Send(fn(buf[:n]))
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

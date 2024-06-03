package command

import (
	"context"
	"io"
	"os"
	"os/exec"
	"reflect"
	"sync"
	"syscall"

	"github.com/creack/pty"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

type virtualCommand struct {
	*base

	isEchoEnabled bool
	logger        *zap.Logger
	stdin         io.ReadCloser // stdin is [CommandOptions.Stdin] wrapped in [readCloser]

	// cmd is populated when the command is started.
	cmd *exec.Cmd

	pty *os.File
	tty *os.File

	wg sync.WaitGroup // watch goroutines copying I/O

	mu  sync.Mutex // protect err
	err error
}

func (c *virtualCommand) getPty() *os.File {
	return c.pty
}

func (c *virtualCommand) Running() bool {
	return c.cmd != nil && c.cmd.ProcessState == nil
}

func (c *virtualCommand) Pid() int {
	if c.cmd == nil || c.cmd.Process == nil {
		return 0
	}
	return c.cmd.Process.Pid
}

func (c *virtualCommand) Stdin() io.Reader {
	return c.stdin
}

func (c *virtualCommand) Start(ctx context.Context) (err error) {
	c.pty, c.tty, err = pty.Open()
	if err != nil {
		return errors.WithStack(err)
	}

	if !c.isEchoEnabled {
		if err := disableEcho(c.tty.Fd()); err != nil {
			return err
		}
	}

	program, args, err := c.ProgramPath()
	if err != nil {
		return err
	}

	c.cmd = exec.CommandContext(
		ctx,
		program,
		args...,
	)
	c.cmd.Dir = c.ProgramConfig().Directory
	c.cmd.Env = c.Env()
	c.cmd.Stdin = c.tty
	c.cmd.Stdout = c.tty
	c.cmd.Stderr = c.tty

	setSysProcAttrCtty(c.cmd)

	c.logger.Info("starting a virtual command", zap.Any("config", redactConfig(c.ProgramConfig())))
	if err := c.cmd.Start(); err != nil {
		return errors.WithStack(err)
	}

	if !isNil(c.stdin) {
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			n, err := io.Copy(c.pty, c.stdin)
			c.logger.Info("finished copying from stdin to pty", zap.Error(err), zap.Int64("count", n))
			if err != nil {
				c.setErr(errors.WithStack(err))
			}
		}()
	}

	stdout := c.Stdout()

	if !isNil(stdout) {
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			n, err := io.Copy(stdout, c.pty)
			if err != nil {
				// Linux kernel returns EIO when attempting to read from
				// a master pseudo-terminal which no longer has an open slave.
				// See https://github.com/creack/pty/issues/21.
				if errors.Is(err, syscall.EIO) {
					c.logger.Debug("failed to copy from pty to stdout; handled EIO")
					return
				}
				if errors.Is(err, os.ErrClosed) {
					c.logger.Debug("failed to copy from pty to stdout; handled ErrClosed")
					return
				}

				c.logger.Info("failed to copy from pty to stdout", zap.Error(err))

				c.setErr(errors.WithStack(err))
			} else {
				c.logger.Debug("finished copying from pty to stdout", zap.Int64("count", n))
			}
		}()
	}

	c.logger.Info("a virtual command started")

	return nil
}

func (c *virtualCommand) Signal(sig os.Signal) error {
	c.logger.Info("stopping the virtual command with signal", zap.String("signal", sig.String()))

	// Try to terminate the whole process group. If it fails, fall back to stdlib methods.
	if err := signalPgid(c.cmd.Process.Pid, sig); err != nil {
		c.logger.Info("failed to terminate process group; trying Process.Signal()", zap.Error(err))
		if err := c.cmd.Process.Signal(sig); err != nil {
			c.logger.Info("failed to signal process; trying Process.Kill()", zap.Error(err))
			return errors.WithStack(c.cmd.Process.Kill())
		}
	}
	return nil
}

func (c *virtualCommand) Wait() (err error) {
	c.logger.Info("waiting for the virtual command to finish")
	err = errors.WithStack(c.cmd.Wait())
	c.logger.Info("the virtual command finished", zap.Error(err))

	errIO := c.closeIO()
	c.logger.Info("closed IO of the virtual command", zap.Error(errIO))
	if err == nil && errIO != nil {
		err = errIO
	}

	c.wg.Wait()

	c.mu.Lock()
	if err == nil && c.err != nil {
		err = c.err
	}
	c.mu.Unlock()

	return
}

func (c *virtualCommand) setErr(err error) {
	if err == nil {
		return
	}
	c.mu.Lock()
	if c.err == nil {
		c.err = err
	}
	c.mu.Unlock()
}

func (c *virtualCommand) closeIO() (err error) {
	if !isNil(c.stdin) {
		if errClose := c.stdin.Close(); errClose != nil {
			err = multierr.Append(err, errors.WithMessage(errClose, "failed to close stdin"))
		}
	}

	if errClose := c.tty.Close(); errClose != nil {
		err = multierr.Append(err, errors.WithMessage(errClose, "failed to close tty"))
	}

	// if err := c.pty.Close(); err != nil {
	// 	return errors.WithMessage(err, "failed to close pty")
	// }

	return
}

type commandWithPty interface {
	getPty() *os.File
}

type Winsize pty.Winsize

func SetWinsize(cmd Command, winsize *Winsize) (err error) {
	cmdPty, ok := cmd.(commandWithPty)
	if !ok {
		return errors.New("winsize: unsupported command type")
	}

	err = pty.Setsize(cmdPty.getPty(), (*pty.Winsize)(winsize))
	return errors.WithStack(err)
}

func isNil(val any) bool {
	if val == nil {
		return true
	}

	v := reflect.ValueOf(val)

	if v.Type().Kind() == reflect.Struct {
		return false
	}

	return reflect.ValueOf(val).IsNil()
}

// readCloser wraps [io.Reader] into [io.ReadCloser].
//
// When Close is called, the underlying read operation is ignored.
// It might discard some read data, or read might hang indefinitely.
// It is caller's responsibility to interrupt the underlying [io.Reader]
// when [virtualCommand] exits.
type readCloser struct {
	r    io.Reader
	done chan struct{}
}

func (r *readCloser) Read(p []byte) (int, error) {
	var (
		n   int
		err error
	)

	readc := make(chan struct{})

	go func() {
		n, err = r.r.Read(p)
		close(readc)
	}()

	select {
	case <-readc:
		return n, err
	case <-r.done:
		return 0, io.EOF
	}
}

func (r *readCloser) Close() error {
	close(r.done)
	return nil
}

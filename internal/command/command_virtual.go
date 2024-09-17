package command

import (
	"context"
	"io"
	"os"
	"os/exec"
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
		c.logger.Info("disabling echo")
		if err := disableEcho(c.tty.Fd()); err != nil {
			return err
		}
	}

	program, args, err := c.ProgramPath()
	if err != nil {
		return err
	}
	c.logger.Info("detected program path and arguments", zap.String("program", program), zap.Strings("args", args))

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

	// Create a new session and set the controlling terminal to tty.
	// The new process group is created automatically so that sending
	// a signal to the command will affect the whole group.
	// 3 is because stdin, stdout, stderr + i-th element in ExtraFiles.
	setSysProcAttrCtty(c.cmd, 3)
	c.cmd.ExtraFiles = []*os.File{c.tty}

	c.logger.Info("starting", zap.Any("config", redactConfig(c.ProgramConfig())))
	if err := c.cmd.Start(); err != nil {
		return errors.WithStack(err)
	}
	c.logger.Info("started")

	if !isNil(c.stdin) {
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			n, err := io.Copy(c.pty, c.stdin)
			if err != nil {
				c.setErr(errors.WithStack(err))
			}
			c.logger.Info("copied from stdin to pty", zap.Error(err), zap.Int64("count", n))
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
					c.logger.Info("failed to copy from pty to stdout; handled EIO")
					return
				}
				if errors.Is(err, os.ErrClosed) {
					c.logger.Info("failed to copy from pty to stdout; handled ErrClosed")
					return
				}
				c.setErr(errors.WithStack(err))
			}

			c.logger.Info("copied from pty to stdout", zap.Int64("count", n))
		}()
	}

	return nil
}

func (c *virtualCommand) Signal(sig os.Signal) error {
	c.logger.Info("stopping with signal", zap.String("signal", sig.String()))

	// Try to terminate the whole process group. If it fails, fall back to stdlib methods.
	err := signalPgid(c.cmd.Process.Pid, sig)
	if err == nil {
		return nil
	}

	c.logger.Info("failed to signal the process group; trying regular signaling", zap.Error(err))

	if err := c.cmd.Process.Signal(sig); err != nil {
		if sig == os.Kill {
			return errors.WithStack(err)
		}
		c.logger.Info("failed to signal the process; trying kill signal", zap.Error(err))
		return errors.WithStack(c.cmd.Process.Kill())
	}

	return nil
}

func (c *virtualCommand) Wait() (err error) {
	c.logger.Info("waiting for finish")
	err = errors.WithStack(c.cmd.Wait())
	c.logger.Info("finished", zap.Error(err))

	errIO := c.closeIO()
	c.logger.Info("closed IO", zap.Error(errIO))
	if err == nil && errIO != nil {
		err = errIO
	}

	c.logger.Info("waiting IO goroutines")
	c.wg.Wait()
	c.logger.Info("finished waiting for IO goroutines")

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

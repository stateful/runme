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

type VirtualCommand struct {
	cfg  *Config
	opts *VirtualCommandOptions

	// cmd is populated when the command is started.
	cmd *exec.Cmd

	// stdin is Opts.Stdin wrapped in readCloser.
	stdin io.ReadCloser

	cleanFuncs []func() error

	pty *os.File
	tty *os.File

	wg sync.WaitGroup

	mu  sync.Mutex
	err error

	logger *zap.Logger
}

// readCloser allows to wrap a io.Reader into io.ReadCloser.
//
// When Close() is called, the underlying read operation is ignored.
// A disadvantage is that it may leak and hang indefinitely, or
// the read data is lost. It's caller's responsibility to interrupt
// the underlying reader when the virtualCommand exits.
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

func newVirtualCommand(cfg *Config, opts *VirtualCommandOptions) *VirtualCommand {
	var stdin io.ReadCloser

	if opts.Stdin != nil {
		stdin = &readCloser{r: opts.Stdin, done: make(chan struct{})}
	}

	// Stdout must be set, otherwise the command will hang trying to copy data from pty.
	if opts.Stdout == nil {
		opts.Stdout = io.Discard
	}

	return &VirtualCommand{
		cfg:    cfg,
		opts:   opts,
		stdin:  stdin,
		logger: opts.Logger,
	}
}

func (c *VirtualCommand) Running() bool {
	return c.cmd != nil && c.cmd.ProcessState == nil
}

func (c *VirtualCommand) Pid() int {
	if c.cmd == nil || c.cmd.Process == nil {
		return 0
	}
	return c.cmd.Process.Pid
}

func (c *VirtualCommand) Start(ctx context.Context) (err error) {
	argsNormalizer := &argsNormalizer{
		session: c.opts.Session,
		logger:  c.logger,
	}

	cfg, err := normalizeConfig(
		c.cfg,
		argsNormalizer,
		&envNormalizer{sources: []envSource{c.opts.Session.GetEnv}},
	)
	if err != nil {
		return
	}

	c.cleanFuncs = append(c.cleanFuncs, argsNormalizer.CollectEnv, argsNormalizer.Cleanup)

	c.pty, c.tty, err = pty.Open()
	if err != nil {
		return errors.WithStack(err)
	}

	if err := disableEcho(c.tty.Fd()); err != nil {
		return err
	}

	c.cmd = exec.CommandContext(
		ctx,
		cfg.ProgramName,
		cfg.Arguments...,
	)
	c.cmd.Dir = cfg.Directory
	c.cmd.Env = cfg.Env
	c.cmd.Stdin = c.tty
	c.cmd.Stdout = c.tty
	c.cmd.Stderr = c.tty

	setSysProcAttrCtty(c.cmd)

	c.logger.Info("starting a virtual command", zap.Any("config", redactConfig(cfg)))
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

	if !isNil(c.opts.Stdout) {
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			n, err := io.Copy(c.opts.Stdout, c.pty)
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

func (c *VirtualCommand) StopWithSignal(sig os.Signal) error {
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

func (c *VirtualCommand) Wait() (err error) {
	c.logger.Info("waiting for the virtual command to finish")

	defer func() {
		errC := errors.WithStack(c.cleanup())
		c.logger.Info("cleaned up the virtual command", zap.Error(errC))
		if err == nil && errC != nil {
			err = errC
		}
	}()

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

func (c *VirtualCommand) setErr(err error) {
	if err == nil {
		return
	}
	c.mu.Lock()
	if c.err == nil {
		c.err = err
	}
	c.mu.Unlock()
}

func (c *VirtualCommand) closeIO() (err error) {
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

func (c *VirtualCommand) cleanup() (err error) {
	for _, fn := range c.cleanFuncs {
		if errFn := fn(); errFn != nil {
			err = multierr.Append(err, errFn)
		}
	}
	return
}

type Winsize pty.Winsize

func SetWinsize(cmd *VirtualCommand, winsize *Winsize) error {
	if cmd.pty == nil {
		return nil
	}
	err := pty.Setsize(cmd.pty, (*pty.Winsize)(winsize))
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

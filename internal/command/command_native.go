package command

import (
	"context"
	"os"
	"os/exec"
	"syscall"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// SignalToProcessGroup is used in tests to disable sending signals to a process group.
var SignalToProcessGroup = true

type NativeCommand struct {
	cfg  *Config
	opts *NativeCommandOptions

	// cmd is populated when the command is started.
	cmd *exec.Cmd

	cleanFuncs []func()

	logger *zap.Logger
}

func newNativeCommand(cfg *Config, opts *NativeCommandOptions) *NativeCommand {
	return &NativeCommand{
		cfg:    cfg,
		opts:   opts,
		logger: opts.Logger,
	}
}

func (c *NativeCommand) Running() bool {
	return c.cmd != nil && c.cmd.ProcessState == nil
}

func (c *NativeCommand) Pid() int {
	if c.cmd == nil || c.cmd.Process == nil {
		return 0
	}
	return c.cmd.Process.Pid
}

func (c *NativeCommand) SetWinsize(rows, cols, x, y uint16) error {
	return errors.New("unsupported")
}

func (c *NativeCommand) Start(ctx context.Context) (err error) {
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

	stdin := c.opts.Stdin

	if f, ok := stdin.(*os.File); ok && f != nil {
		// Duplicate /dev/stdin.
		newStdinFd, err := dup(int(f.Fd()))
		if err != nil {
			return errors.Wrap(err, "failed to dup stdin")
		}
		syscall.CloseOnExec(newStdinFd)

		// Setting stdin to the non-block mode fails on the simple "read" command.
		// On the other hand, it allows to use SetReadDeadline().
		// It turned out it's not needed, but keeping the code here for now.
		// if err := syscall.SetNonblock(newStdinFd, true); err != nil {
		// 	return nil, errors.Wrap(err, "failed to set new stdin fd in non-blocking mode")
		// }

		stdin = os.NewFile(uintptr(newStdinFd), "")
	}

	c.cmd = exec.CommandContext(
		ctx,
		cfg.ProgramName,
		cfg.Arguments...,
	)
	c.cmd.Dir = cfg.Directory
	c.cmd.Env = cfg.Env
	c.cmd.Stdin = stdin
	c.cmd.Stdout = c.opts.Stdout
	c.cmd.Stderr = c.opts.Stderr

	// Set the process group ID of the program.
	// It is helpful to stop the program and its
	// children.
	// Note that Setsid set in setSysProcAttrCtty()
	// already starts a new process group.
	// Warning: it does not work with interactive programs
	// like "python", hence, it's commented out.
	// setSysProcAttrPgid(c.cmd)

	c.logger.Info("starting a local command", zap.Any("config", redactConfig(cfg)))

	if err := c.cmd.Start(); err != nil {
		return errors.WithStack(err)
	}

	c.logger.Info("a local command started")

	return nil
}

func (c *NativeCommand) StopWithSignal(sig os.Signal) error {
	if SignalToProcessGroup {
		// Try to terminate the whole process group. If it fails, fall back to stdlib methods.
		err := signalPgid(c.cmd.Process.Pid, sig)
		if err == nil {
			return nil
		}
		c.logger.Info("failed to terminate process group; trying Process.Signal()", zap.Error(err))
	}

	if err := c.cmd.Process.Signal(sig); err != nil {
		c.logger.Info("failed to signal process; trying Process.Kill()", zap.Error(err))
		return errors.WithStack(c.cmd.Process.Kill())
	}

	return nil
}

func (c *NativeCommand) Wait() error {
	c.logger.Info("waiting for the local command to finish")

	defer c.cleanup()

	var stderr []byte

	err := c.cmd.Wait()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			stderr = exitErr.Stderr
		}
	}

	c.logger.Info("the local command finished", zap.Error(err), zap.ByteString("stderr", stderr))

	return errors.WithStack(err)
}

func (c *NativeCommand) cleanup() {
	for _, fn := range c.cleanFuncs {
		fn()
	}
}

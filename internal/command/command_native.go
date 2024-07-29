package command

import (
	"context"
	"os"
	"os/exec"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type nativeCommand struct {
	*base

	disableNewProcessID bool
	logger              *zap.Logger

	cmd *exec.Cmd
}

func (c *nativeCommand) Running() bool {
	return c.cmd != nil && c.cmd.ProcessState == nil
}

func (c *nativeCommand) Pid() int {
	if c.cmd == nil || c.cmd.Process == nil {
		return 0
	}
	return c.cmd.Process.Pid
}

func (c *nativeCommand) Start(ctx context.Context) (err error) {
	stdin := c.Stdin()

	// TODO(adamb): include explanation why it is needed.
	if f, ok := stdin.(*os.File); ok && f != nil {
		// Duplicate /dev/stdin.
		newStdinFd, err := dup(f.Fd())
		if err != nil {
			return errors.Wrap(err, "failed to dup stdin")
		}
		closeOnExec(newStdinFd)
		stdin = os.NewFile(uintptr(newStdinFd), "")
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
	c.cmd.Stdin = stdin
	c.cmd.Stdout = c.Stdout()
	c.cmd.Stderr = c.Stderr()

	if !c.disableNewProcessID {
		// Creating a new process group is required to properly replicate a behaviour
		// similar to CTRL-C in the terminal, which sends a SIGINT to the whole group.
		setSysProcAttrPgid(c.cmd)
	}

	c.logger.Info("starting", zap.Any("config", redactConfig(c.ProgramConfig())))
	if err := c.cmd.Start(); err != nil {
		return errors.WithStack(err)
	}
	c.logger.Info("started")

	return nil
}

func (c *nativeCommand) Signal(sig os.Signal) error {
	c.logger.Info("stopping with signal", zap.Stringer("signal", sig))

	if !c.disableNewProcessID {
		c.logger.Info("signaling to the process group", zap.Stringer("signal", sig))
		// Try to terminate the whole process group. If it fails, fall back to stdlib methods.
		err := signalPgid(c.cmd.Process.Pid, sig)
		if err == nil {
			return nil
		}
		c.logger.Info("failed to signal the process group; trying regular signaling", zap.Error(err))
	}

	if err := c.cmd.Process.Signal(sig); err != nil {
		if sig == os.Kill {
			return errors.WithStack(err)
		}
		c.logger.Info("failed to signal the process; trying kill signal", zap.Error(err))
		return errors.WithStack(c.cmd.Process.Kill())
	}

	return nil
}

func (c *nativeCommand) Wait() (err error) {
	c.logger.Info("waiting for finish")

	var stderr []byte
	err = errors.WithStack(c.cmd.Wait())
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			stderr = exitErr.Stderr
		}
	}

	c.logger.Info("finished", zap.Error(err), zap.ByteString("stderr", stderr))

	return
}

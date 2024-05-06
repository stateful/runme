package command

import (
	"context"
	"os"
	"os/exec"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// SignalToProcessGroup is used in tests to disable sending signals to a process group.
var SignalToProcessGroup = true

type nativeCommand struct {
	internalCommand

	cmd *exec.Cmd
}

var _ Command = (*nativeCommand)(nil)

func newNative(cfg *ProgramConfig, opts Options) *nativeCommand {
	return &nativeCommand{
		internalCommand: newBase(cfg, opts),
	}
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
	logger := c.Logger()
	stdin := c.Stdin()

	if f, ok := stdin.(*os.File); ok && f != nil {
		// Duplicate /dev/stdin.
		newStdinFd, err := dup(f.Fd())
		if err != nil {
			return errors.Wrap(err, "failed to dup stdin")
		}
		closeOnExec(newStdinFd)

		// Setting stdin to the non-block mode fails on the simple "read" command.
		// On the other hand, it allows to use SetReadDeadline().
		// It turned out it's not needed, but keeping the code here for now.
		// if err := syscall.SetNonblock(newStdinFd, true); err != nil {
		// 	return nil, errors.Wrap(err, "failed to set new stdin fd in non-blocking mode")
		// }

		stdin = os.NewFile(uintptr(newStdinFd), "")
	}

	program, args, err := c.ProgramPath()
	if err != nil {
		return err
	}
	logger.Info("detected program path and arguments", zap.String("program", program), zap.Strings("args", args))

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

	// Set the process group ID of the program.
	// It is helpful to stop the program and its
	// children.
	// Note that Setsid set in setSysProcAttrCtty()
	// already starts a new process group.
	// Warning: it does not work with interactive programs
	// like "python", hence, it's commented out.
	// setSysProcAttrPgid(c.cmd)

	logger.Info("starting a native command", zap.Any("config", redactConfig(c.ProgramConfig())))
	if err := c.cmd.Start(); err != nil {
		return errors.WithStack(err)
	}
	logger.Info("a native command started")

	return nil
}

func (c *nativeCommand) Signal(sig os.Signal) error {
	logger := c.Logger()

	logger.Info("stopping the native command with a signal", zap.Stringer("signal", sig))

	if SignalToProcessGroup {
		// Try to terminate the whole process group. If it fails, fall back to stdlib methods.
		err := signalPgid(c.cmd.Process.Pid, sig)
		if err == nil {
			return nil
		}
		logger.Info("failed to terminate process group; trying Process.Signal()", zap.Error(err))
	}

	if err := c.cmd.Process.Signal(sig); err != nil {
		logger.Info("failed to signal process; trying Process.Kill()", zap.Error(err))
		return errors.WithStack(c.cmd.Process.Kill())
	}

	return nil
}

func (c *nativeCommand) Wait() (err error) {
	logger := c.Logger()

	logger.Info("waiting for the native command to finish")
	var stderr []byte
	err = errors.WithStack(c.cmd.Wait())
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			stderr = exitErr.Stderr
		}
	}
	logger.Info("the native command finished", zap.Error(err), zap.ByteString("stderr", stderr))

	return
}

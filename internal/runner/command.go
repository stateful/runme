package runner

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/creack/pty"
	"github.com/pkg/errors"
	"github.com/stateful/runme/internal/rbuffer"
	"go.uber.org/zap"
)

type command struct {
	ProgramPath string
	Args        []string
	Directory   string
	Envs        []string
	Stdin       io.ReadWriter
	Stdout      io.ReadWriteCloser
	Stderr      io.ReadWriteCloser

	cfg *commandConfig

	// pty and tty as pseud-terminal primary and secondary.
	// Might be nil if not allocating a pseudo-terminal.
	pty *os.File
	tty *os.File

	cmd *exec.Cmd

	done chan struct{}
	wg   sync.WaitGroup
	mu   sync.Mutex
	err  error

	logger *zap.Logger
}

type commandConfig struct {
	ProgramName string   // a path to shell or a name, for example: "/usr/local/bin/bash", "bash"
	Args        []string // args passed to the program
	Directory   string
	Envs        []string
	Tty         bool // if true, a pseudo-terminal is allocated

	IsShell  bool // if true then Commands or Scripts is passed to shell as "-c" argument's value
	Commands []string
	Script   string

	Input []byte // initial input data passed immediately to the command.
}

func newCommand(
	cfg *commandConfig,
	logger *zap.Logger,
) (*command, error) {
	programPath, err := exec.LookPath(cfg.ProgramName)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	directory := cfg.Directory
	if directory == "" {
		var err error
		directory, err = os.Getwd()
		if err != nil {
			return nil, errors.WithStack(err)
		}
	}

	var extraArgs []string

	if cfg.IsShell {
		script := ""
		if len(cfg.Commands) > 0 {
			script = prepareScriptFromCommands(cfg.Commands)
		} else if cfg.Script != "" {
			script = prepareScript(cfg.Script)
		}
		if script != "" {
			extraArgs = []string{"-c", script}
		}
	}

	cmd := &command{
		ProgramPath: programPath,
		Args:        append(cfg.Args, extraArgs...),
		Directory:   directory,
		Envs:        cfg.Envs,
		cfg:         cfg,
		done:        make(chan struct{}),
		logger:      logger,
	}

	if cfg.Tty {
		var err error
		cmd.pty, cmd.tty, err = pty.Open()
		if err != nil {
			return nil, errors.WithStack(err)
		}

		cmd.Stdin = cmd.pty
		if len(cfg.Input) > 0 {
			_, err := io.Copy(cmd.Stdin, bytes.NewReader(cfg.Input))
			if err != nil {
				return nil, errors.WithMessage(err, "failed to write initial input")
			}
		}

		// stdout is read from pty. stderr is unused because
		// it can't be distinguished from pty.
		cmd.Stdout = rbuffer.NewRingBuffer(4096)
	} else {
		cmd.Stdin = &safeBuffer{buf: bytes.NewBuffer(cfg.Input)}
		cmd.Stdout = rbuffer.NewRingBuffer(4096)
		cmd.Stderr = rbuffer.NewRingBuffer(4096)
	}

	return cmd, nil
}

func (c *command) Start(ctx context.Context) error {
	cmd := exec.CommandContext(
		ctx,
		c.ProgramPath,
		c.Args...,
	)
	cmd.Dir = c.Directory
	cmd.Env = append(cmd.Env, c.Envs...)

	if c.tty != nil {
		cmd.Stdin = c.tty
		cmd.Stdout = c.tty
		cmd.Stderr = c.tty
	} else {
		cmd.Stdin = c.Stdin
		cmd.Stdout = c.Stdout
		cmd.Stderr = c.Stderr
	}

	if c.cfg.Tty {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setsid:  true,
			Setctty: true,
		}
	}

	c.cmd = cmd

	if err := cmd.Start(); err != nil {
		return errors.WithStack(err)
	}

	if c.tty != nil {
		_ = c.tty.Close() // not needed to be open anymore
	}

	if c.pty != nil {
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			_, err := io.Copy(c.Stdout, c.pty)
			if err != nil {
				c.logger.Info("failed to copy from pty to stdout", zap.Error(err))
				c.mu.Lock()
				if c.err == nil {
					c.err = err
				}
				c.mu.Unlock()
			}
		}()
	}

	return nil
}

func (c *command) Wait() error {
	err := c.cmd.Wait()

	if c.pty != nil {
		_ = c.pty.Close()
	}
	if c.Stdout != nil {
		_ = c.Stdout.Close()
	}
	if c.Stderr != nil {
		_ = c.Stderr.Close()
	}

	c.wg.Wait()

	if err == nil {
		c.mu.Lock()
		if c.err != nil {
			err = c.err
		}
		c.mu.Unlock()
	}

	return errors.WithStack(err)
}

func exitCodeFromErr(err error) int {
	if err == nil {
		return 0
	}
	var exiterr *exec.ExitError
	if errors.As(err, &exiterr) {
		return exiterr.ExitCode()
	}
	return -1
}

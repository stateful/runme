package runner

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/creack/pty"
	"github.com/pkg/errors"
	"github.com/stateful/runme/internal/rbuffer"
	"go.uber.org/multierr"
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

	envDir string

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

	var (
		extraArgs    []string
		envStorePath string
	)

	if cfg.IsShell && (len(cfg.Commands) > 0 || cfg.Script != "") {
		var err error
		envStorePath, err = os.MkdirTemp("", "")
		if err != nil {
			return nil, errors.WithStack(err)
		}

		var script strings.Builder

		_, _ = script.WriteString(fmt.Sprintf("env > %s\n", filepath.Join(envStorePath, ".env_start")))

		if len(cfg.Commands) > 0 {
			_, _ = script.WriteString(prepareScriptFromCommands(cfg.Commands))
		} else if cfg.Script != "" {
			_, _ = script.WriteString(prepareScript(cfg.Script))
		}

		_, _ = script.WriteString(fmt.Sprintf("env > %s\n", filepath.Join(envStorePath, ".env_end")))

		extraArgs = []string{"-c", script.String()}
	}

	cmd := &command{
		ProgramPath: programPath,
		Args:        append(cfg.Args, extraArgs...),
		Directory:   directory,
		Envs:        cfg.Envs,
		cfg:         cfg,
		envDir:      envStorePath,
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
				cmd.cleanup()
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
		setCmdAttrs(cmd)
	}

	c.cmd = cmd

	if err := cmd.Start(); err != nil {
		c.cleanup()
		return errors.WithStack(err)
	}

	if c.tty != nil {
		if err := c.tty.Close(); err != nil {
			c.logger.Info("failed to close tty after starting the command", zap.Error(err))
		} else {
			c.tty = nil
		}
	}

	if c.pty != nil {
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			_, err := io.Copy(c.Stdout, c.pty)
			if err != nil {
				// Linux kernel returns EIO when attempting to read from
				// a master pseudo-terminal which no longer has an open slave.
				// See https://github.com/creack/pty/issues/21.
				if errors.Is(err, syscall.EIO) {
					c.logger.Debug("failed to copy from pty to stdout; handled EIO error")
					return
				}

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

func (c *command) cleanup() {
	var err error

	if c.envDir != "" {
		if e := os.RemoveAll(c.envDir); e != nil {
			c.logger.Info("failed to delete envsDir", zap.Error(e))
			err = multierr.Append(err, e)
		}
	}
	if c.tty != nil {
		if e := c.tty.Close(); e != nil {
			c.logger.Info("failed to close tty", zap.Error(e))
			err = multierr.Append(err, e)
		}
	}
	if c.pty != nil {
		if e := c.pty.Close(); err != nil {
			c.logger.Info("failed to close pty", zap.Error(e))
			err = multierr.Append(err, e)
		}
	}
	if c.Stdout != nil {
		if e := c.Stdout.Close(); e != nil {
			c.logger.Info("failed to close stdout", zap.Error(e))
			err = multierr.Append(err, e)
		}
	}
	if c.Stderr != nil {
		if e := c.Stderr.Close(); e != nil {
			c.logger.Info("failed to close stderr", zap.Error(e))
			err = multierr.Append(err, e)
		}
	}

	c.mu.Lock()
	if c.err == nil {
		c.err = err
	}
	c.mu.Unlock()
}

func (c *command) readEnvFromFile(name string) (result []string, _ error) {
	f, err := os.Open(filepath.Join(c.envDir, name))
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		result = append(result, scanner.Text())
	}

	return result, errors.WithStack(scanner.Err())
}

func (c *command) collectEnvs() error {
	if c.envDir == "" {
		return nil
	}

	startEnvs, err := c.readEnvFromFile(".env_start")
	if err != nil {
		return err
	}

	endEnvs, err := c.readEnvFromFile(".env_end")
	if err != nil {
		return err
	}

	newOrUpdated, _, deleted := diffEnvStores(
		newEnvStore(startEnvs...),
		newEnvStore(endEnvs...),
	)

	c.Envs = newEnvStore(c.Envs...).Add(newOrUpdated...).Delete(deleted...).Values()

	return nil
}

func (c *command) Wait() error {
	werr := c.cmd.Wait()

	eerr := c.collectEnvs()

	c.cleanup()

	c.wg.Wait()

	if werr != nil {
		return werr
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return c.err
	}

	if eerr != nil {
		return eerr
	}

	return nil
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

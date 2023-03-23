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
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

const (
	envStartFileName = ".env_start"
	envEndFileName   = ".env_end"
)

var dumpCmd = getDumpCmd()

type command struct {
	ProgramPath string
	Args        []string
	Directory   string
	Session     *Session
	Stdin       io.Reader
	Stdout      io.Writer
	Stderr      io.Writer

	cmd *exec.Cmd

	// pty and tty as pseud-terminal primary and secondary.
	// Might be nil if not allocating a pseudo-terminal.
	pty *os.File
	tty *os.File

	tmpEnvDir string

	wg  sync.WaitGroup
	mu  sync.Mutex
	err error

	logger *zap.Logger
}

func (c *command) seterr(err error) {
	if err == nil {
		return
	}
	c.mu.Lock()
	if c.err == nil {
		c.err = err
	}
	c.mu.Unlock()
}

type commandConfig struct {
	ProgramName string   // a path to shell or a name, for example: "/usr/local/bin/bash", "bash"
	Args        []string // args passed to the program
	Directory   string
	Session     *Session

	Tty     bool // if true, a pseudo-terminal is allocated
	Winsize *pty.Winsize

	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	IsShell  bool // if true then Commands or Scripts is passed to shell as "-c" argument's value
	Commands []string
	Script   string

	Logger *zap.Logger
}

func newCommand(cfg *commandConfig) (*command, error) {
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

		_, _ = script.WriteString(fmt.Sprintf("%s > %s\n", dumpCmd, filepath.Join(envStorePath, envStartFileName)))

		if len(cfg.Commands) > 0 {
			_, _ = script.WriteString(
				prepareScriptFromCommands(cfg.Commands, ShellFromShellPath(programPath)),
			)
		} else if cfg.Script != "" {
			_, _ = script.WriteString(
				prepareScript(cfg.Script, ShellFromShellPath(programPath)),
			)
		}

		_, _ = script.WriteString(fmt.Sprintf("%s > %s\n", dumpCmd, filepath.Join(envStorePath, envEndFileName)))

		extraArgs = []string{"-c", script.String()}
	}

	session := cfg.Session
	if session == nil {
		session = NewSession(nil, cfg.Logger)
	}

	cmd := &command{
		ProgramPath: programPath,
		Args:        append(cfg.Args, extraArgs...),
		Directory:   directory,
		Session:     session,
		Stdin:       cfg.Stdin,
		Stdout:      cfg.Stdout,
		Stderr:      cfg.Stderr,
		tmpEnvDir:   envStorePath,
		logger:      cfg.Logger,
	}

	if cfg.Tty {
		var err error
		cmd.pty, cmd.tty, err = pty.Open()
		if err != nil {
			cmd.cleanup()
			return nil, errors.WithStack(err)
		}
		if cfg.Winsize != nil {
			cmd.setWinsize(cfg.Winsize)
		}
	}

	return cmd, nil
}

func (c *command) cleanup() {
	var err error

	if c.tmpEnvDir != "" {
		if e := os.RemoveAll(c.tmpEnvDir); e != nil {
			c.logger.Info("failed to delete tmpEnvDir", zap.Error(e))
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

	c.seterr(err)
}

type startOpts struct {
	DisableEcho bool
}

func (c *command) Start(ctx context.Context) error {
	return c.StartWithOpts(ctx, &startOpts{})
}

func (c *command) StartWithOpts(ctx context.Context, opts *startOpts) error {
	c.cmd = exec.CommandContext(
		ctx,
		c.ProgramPath,
		c.Args...,
	)
	c.cmd.Dir = c.Directory
	c.cmd.Env = append(c.cmd.Env, c.Session.Envs()...)

	if c.tty != nil {
		c.cmd.Stdin = c.tty
		c.cmd.Stdout = c.tty
		c.cmd.Stderr = c.tty

		setSysProcAttrCtty(c.cmd)
	} else {
		c.cmd.Stdin = c.Stdin
		c.cmd.Stdout = c.Stdout
		c.cmd.Stderr = c.Stderr

		// Set the process group ID of the program.
		// It is helpful to stop the program and its
		// children. See command.Stop().
		// Note that Setsid set in setSysProcAttrCtty()
		// already starts a new process group, hence,
		// this call is inside this branch.
		setSysProcAttrPgid(c.cmd)
	}

	if err := c.cmd.Start(); err != nil {
		c.cleanup()
		return errors.WithStack(err)
	}

	if c.tty != nil {
		if opts.DisableEcho {
			// Disable echoing. This solves the problem of duplicating entered line in the output.
			// This is one of the solutions, but there are other methods:
			//   - removing echoed input from the output
			//   - removing the entered line using escape codes
			if err := disableEcho(c.tty.Fd()); err != nil {
				return err
			}
		}

		// Close tty as not needed anymore.
		if err := c.tty.Close(); err != nil {
			c.logger.Info("failed to close tty after starting the command", zap.Error(err))
		}

		c.tty = nil
	}

	if c.pty != nil {
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			n, err := io.Copy(c.pty, c.Stdin)
			if err != nil {
				c.logger.Info("failed to copy from stdin to pty", zap.Error(err))
				c.seterr(err)
			} else {
				c.logger.Debug("finished copying from stdin to pty", zap.Int64("count", n))
			}
		}()

		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			n, err := io.Copy(c.Stdout, c.pty)
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

				c.seterr(err)
			} else {
				c.logger.Debug("finished copying from pty to stdout", zap.Int64("count", n))
			}
		}()
	}

	return nil
}

func (c *command) Kill() error {
	return c.stop(os.Kill)
}

func (c *command) StopWithSignal(sig os.Signal) error {
	return c.stop(sig)
}

func (c *command) stop(sig os.Signal) error {
	if c.cmd == nil {
		return errors.New("command not started")
	}

	if c.pty != nil {
		if err := c.pty.Close(); err != nil {
			c.logger.Info("failed to close pty; continuing")
		}
	}

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

func (c *command) readEnvFromFile(name string) (result []string, _ error) {
	f, err := os.Open(filepath.Join(c.tmpEnvDir, name))
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	scanner.Split(splitNull)

	for scanner.Scan() {
		result = append(result, scanner.Text())
	}

	return result, errors.WithStack(scanner.Err())
}

func splitNull(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, 0); i >= 0 {
		// We have a full null-terminated line.
		return i + 1, data[0:i], nil
	}
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), data, nil
	}
	// Request more data.
	return 0, nil, nil
}

func (c *command) collectEnvs() {
	if c.tmpEnvDir == "" {
		return
	}

	startEnvs, err := c.readEnvFromFile(envStartFileName)
	c.seterr(err)

	endEnvs, err := c.readEnvFromFile(envEndFileName)
	c.seterr(err)

	newOrUpdated, _, deleted := diffEnvStores(
		newEnvStore(startEnvs...),
		newEnvStore(endEnvs...),
	)

	c.Session.envStore = newEnvStore(c.cmd.Env...).Add(newOrUpdated...).Delete(deleted...)
}

// ProcessWait waits only for the process to exit.
// You rather want to use Wait().
func (c *command) ProcessWait() error {
	return errors.WithStack(c.cmd.Wait())
}

// Finalize performs necassary actions and cleanups after the process exits.
func (c *command) Finalize() (err error) {
	if c.cmd.ProcessState == nil {
		return errors.New("process not finished")
	}

	// TODO(adamb): when collecting envs is improved,
	// this condition might be not needed anymore.
	if c.cmd.ProcessState.Success() {
		c.collectEnvs()
	}

	c.cleanup()

	c.wg.Wait()

	c.mu.Lock()
	err = c.err
	c.mu.Unlock()
	return
}

// Wait waits for the process to exit as well as all its goroutines.
func (c *command) Wait() error {
	if err := c.ProcessWait(); err != nil {
		return err
	}
	return c.Finalize()
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

func (c *command) setWinsize(winsize *pty.Winsize) {
	if c.pty == nil {
		return
	}

	_ = pty.Setsize(c.pty, winsize)
}

func getDumpCmd() string {
	path, _ := os.Executable()
	return strings.Join([]string{path, "env", "dump", "--insecure"}, " ")
}

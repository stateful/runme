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
	"sync/atomic"
	"syscall"

	"github.com/creack/pty"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/stateful/runme/v3/internal/system"
	"github.com/stateful/runme/v3/internal/ulid"
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

	cmd     *exec.Cmd
	cmdDone uint32

	// pty and tty as pseud-terminal primary and secondary.
	// Might be nil if not allocating a pseudo-terminal.
	pty *os.File
	tty *os.File

	tmpEnvDir string

	tempScriptFile string

	context context.Context
	wg      sync.WaitGroup
	mu      sync.Mutex
	err     error

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

type CommandMode uint

const (
	// Do not send script to program
	CommandModeNone CommandMode = iota
	// Send script as argument to "-c"
	CommandModeInlineShell
	// Send script as temporary file in cwd
	CommandModeTempFile
)

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

	Commands []string
	Script   string

	CommandMode   CommandMode
	LanguageID    string
	FileExtension string

	Logger *zap.Logger
}

func newCommand(context context.Context, cfg *commandConfig) (*command, error) {
	var pathEnv string

	// If PATH is set in the session, use it in the system
	// so that program paths can be resolved correctly.
	// This is especially important for virtual envs.
	if cfg.Session != nil {
		if path, err := cfg.Session.envStorer.getEnv("PATH"); err == nil && path != "" {
			pathEnv = path
		}
	}

	programName, initialArgs := parseFileProgram(cfg.ProgramName)
	args := initialArgs

	programPath, initialArgs, err := inferFileProgram(pathEnv, programName, cfg.LanguageID)
	args = append(args, initialArgs...)
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

	isShell := false

	var tempScriptFile string

	fileExtension := cfg.FileExtension
	if fileExtension == "" {
		fileExtension = inferFileExtension(cfg.LanguageID)
	}

	var tmpEnvDir string

	for _, candidate := range []string{
		"bash", "sh", "ksh", "zsh", "fish", "powershell", "pwsh", "cmd",
	} {
		cmdName := filepath.Base(programPath)

		if cmdName == candidate {
			isShell = true
		}
	}

	if cfg.CommandMode != CommandModeNone && (len(cfg.Commands) > 0 || cfg.Script != "") {
		var err error
		envStorePath, err = os.MkdirTemp("", "")
		if err != nil {
			return nil, errors.WithStack(err)
		}

		var script string

		if isShell {
			var builder strings.Builder

			tmpEnvDir = envStorePath

			_, _ = builder.WriteString(fmt.Sprintf("%s > %s\n", dumpCmd, filepath.Join(envStorePath, envStartFileName)))
			_, _ = builder.WriteString(fmt.Sprintf("__cleanup() {\nrv=$?\n%s > %s\nexit $rv\n}\n", dumpCmd, filepath.Join(envStorePath, envEndFileName)))
			_, _ = builder.WriteString("trap -- \"__cleanup\" EXIT\n")

			if len(cfg.Commands) > 0 {
				_, _ = builder.WriteString(
					prepareScriptFromCommands(cfg.Commands, ShellFromShellPath(programPath)),
				)
			} else if cfg.Script != "" {
				_, _ = builder.WriteString(
					prepareScript(cfg.Script, ShellFromShellPath(programPath)),
				)
			}

			extraArgs = []string{}

			if cfg.Tty {
				extraArgs = append(extraArgs, "-i")
			}

			script = builder.String()
		} else {
			if len(cfg.Commands) > 0 {
				script = strings.Join(cfg.Commands, "\n")
			} else {
				script = cfg.Script
			}
		}

		switch cfg.CommandMode {
		case CommandModeInlineShell:
			extraArgs = append(extraArgs, "-c", script)
		case CommandModeTempFile:
			for {
				id := ulid.GenerateID()
				tempScriptFile = filepath.Join(cfg.Directory, fmt.Sprintf(".runme-script-%s", id))

				if fileExtension != "" {
					tempScriptFile += "." + fileExtension
				}

				_, err = os.Stat(tempScriptFile)

				if os.IsNotExist(err) {
					err = os.WriteFile(tempScriptFile, []byte(script), 0o600)
					if err != nil {
						return nil, err
					}

					break
				} else if err != nil {
					return nil, err
				}
			}

			extraArgs = append(extraArgs, tempScriptFile)
		}
	}

	session := cfg.Session
	if session == nil {
		// todo(sebastian): owl store?
		session, err = NewSession(nil, nil, cfg.Logger)
		if err != nil {
			return nil, err
		}
	}

	args = append(args, cfg.Args...)

	cmd := &command{
		context:        context,
		ProgramPath:    programPath,
		Args:           append(args, extraArgs...),
		Directory:      directory,
		Session:        session,
		Stdin:          cfg.Stdin,
		Stdout:         cfg.Stdout,
		Stderr:         cfg.Stderr,
		logger:         cfg.Logger,
		tempScriptFile: tempScriptFile,
		tmpEnvDir:      tmpEnvDir,
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

	if c.tempScriptFile != "" {
		if e := os.Remove(c.tempScriptFile); e != nil {
			c.logger.Info("failed to delete tempScriptFile", zap.Error(e))
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
	env, err := c.Session.Envs()
	if err != nil {
		return err
	}
	c.cmd.Env = append(c.cmd.Env, env...)

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
		if sig == os.Interrupt {
			_, _ = c.pty.Write([]byte{0x3})
		}

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

	fileInfo, err := f.Stat()
	if err != nil {
		return nil, err
	}

	numBytes := fileInfo.Size()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, numBytes), int(numBytes))
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

	err = c.Session.UpdateStore(c.context, c.cmd.Env, newOrUpdated, deleted)
	c.seterr(err)
}

func (c *command) ProcessFinished() bool {
	return atomic.LoadUint32(&c.cmdDone) == 1
}

// ProcessWait waits only for the process to exit.
// You rather want to use Wait().
func (c *command) ProcessWait() error {
	err := c.cmd.Wait()
	atomic.StoreUint32(&c.cmdDone, 1)
	return errors.WithStack(err)
}

// Finalize performs necessary actions and cleanups after the process exits.
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

var fileExtensionByLanguageID = map[string]string{
	"js":              "js",
	"javascript":      "js",
	"jsx":             "jsx",
	"javascriptreact": "jsx",
	"tsx":             "tsx",
	"typescriptreact": "tsx",
	"typescript":      "ts",
	"ts":              "ts",
	"sh":              "sh",
	"bash":            "sh",
	"ksh":             "sh",
	"zsh":             "sh",
	"fish":            "sh",
	"powershell":      "ps1",
	"cmd":             "bat",
	"dos":             "bat",
	"py":              "py",
	"python":          "py",
	"ruby":            "rb",
	"rb":              "rb",
}

func inferFileExtension(languageID string) string {
	return fileExtensionByLanguageID[languageID]
}

var programByLanguageID = map[string][]string{
	"js":              {"node"},
	"javascript":      {"node"},
	"jsx":             {"node"},
	"javascriptreact": {"node"},

	"ts":              {"ts-node", "deno run", "bun run"},
	"typescript":      {"ts-node", "deno run", "bun run"},
	"tsx":             {"ts-node", "deno run", "bun run"},
	"typescriptreact": {"ts-node", "deno run", "bun run"},

	"sh":          {"bash", "sh"},
	"bash":        {"bash", "sh"},
	"ksh":         {"ksh"},
	"zsh":         {"zsh"},
	"fish":        {"fish"},
	"powershell":  {"powershell"},
	"cmd":         {"cmd"},
	"dos":         {"cmd"},
	"shellscript": {"bash", "sh"},
	"lua":         {"lua"},
	"perl":        {"perl"},
	"php":         {"php"},
	"python":      {"python3", "python"},
	"py":          {"python3", "python"},
	"ruby":        {"ruby"},
	"rb":          {"ruby"},
}

type ErrInvalidLanguage struct {
	LanguageID string
}

func (e ErrInvalidLanguage) Error() string {
	return fmt.Sprintf("unsupported language %s", e.LanguageID)
}

type ErrInvalidProgram struct {
	Program string
	inner   error
}

func (e ErrInvalidProgram) Error() string {
	return fmt.Sprintf("unable to locate program %q", e.Program)
}

func (e ErrInvalidProgram) Unwrap() error {
	return e.inner
}

// TODO: do this with shlex?
func parseFileProgram(programPath string) (program string, args []string) {
	parts := strings.SplitN(programPath, " ", 2)

	if len(parts) > 0 {
		program = parts[0]
	}

	if len(parts) > 1 {
		args = []string{parts[1]}
	}

	return
}

func inferFileProgram(pathEnv, programPath, languageID string) (interpreter string, args []string, err error) {
	if programPath != "" {
		res, err := system.LookPathUsingPathEnv(programPath, pathEnv)
		if err != nil {
			return "", []string{}, &ErrInvalidProgram{
				Program: programPath,
				inner:   err,
			}
		}
		return res, []string{}, nil
	}

	for _, candidate := range programByLanguageID[languageID] {
		program, args := parseFileProgram(candidate)
		res, err := system.LookPathUsingPathEnv(program, pathEnv)
		if err == nil {
			return res, args, nil
		}
	}

	// Default to "cat"
	res, err := system.LookPathUsingPathEnv("cat", pathEnv)
	if err == nil {
		return res, args, nil
	}

	return "", []string{}, &ErrInvalidLanguage{LanguageID: languageID}
}

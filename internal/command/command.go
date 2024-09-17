package command

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/stateful/runme/v3/pkg/project"
)

type Command interface {
	Interactive() bool
	Pid() int
	Running() bool
	Start(context.Context) error
	Signal(os.Signal) error
	Wait() error
}

type internalCommandGetters interface {
	Env() []string
	ProgramConfig() *ProgramConfig
	ProgramPath() (string, []string, error)
	Stdin() io.Reader
	Stdout() io.Writer
	Stderr() io.Writer
}

type internalCommand interface {
	Command
	internalCommandGetters
}

type base struct {
	cfg     *ProgramConfig
	logger  *zap.Logger
	project *project.Project
	runtime Runtime
	session *Session
	stdin   io.Reader
	stdout  io.Writer
	stderr  io.Writer
}

var _ internalCommand = (*base)(nil)

func (c *base) Interactive() bool {
	return c.cfg.Interactive
}

func (c *base) Pid() int {
	return -1
}

func (c *base) Running() bool {
	return false
}

func (c *base) Start(context.Context) error {
	return errors.New("not implemented")
}

func (c *base) Signal(os.Signal) error {
	return errors.New("not implemented")
}

func (c *base) Wait() error {
	return errors.New("not implemented")
}

func (c *base) Env() []string {
	env := c.runtime.Environ()

	if c.project != nil {
		projEnv, err := c.project.LoadEnv()
		if err != nil {
			c.logger.Warn("failed to load project env", zap.Error(err))
		}
		env = append(env, projEnv...)
	}

	env = append(env, c.session.GetAllEnv()...)
	env = append(env, c.cfg.Env...)

	if err := c.limitEnviron(env); err != nil {
		c.logger.Error("environment size exceeds the limit", zap.Error(err))
	}

	return env
}

func (c *base) ProgramConfig() *ProgramConfig {
	return c.cfg
}

func (c *base) ProgramPath() (string, []string, error) {
	if c.cfg.ProgramName != "" {
		return c.findProgramInPath(c.cfg.ProgramName, c.cfg.Arguments)
	}

	// If language ID is empty, interpreter lookup is futile.
	if c.cfg.LanguageId != "" {
		path, args, err := c.findProgramInKnownInterpreters(c.cfg.LanguageId, c.cfg.Arguments)
		if err == nil {
			return path, args, nil
		}
	}

	return c.findDefaultProgram(c.cfg.ProgramName, c.cfg.Arguments)
}

func (c *base) limitEnviron(environ []string) error {
	const stdoutPrefix = StoreStdoutEnvName + "="

	stdoutEnvIdx := -1
	size := 0
	for idx, e := range environ {
		size += len(e)

		if strings.HasPrefix(e, stdoutPrefix) {
			stdoutEnvIdx = idx
		}
	}

	if size <= MaxEnvironSizeInBytes {
		return nil
	}

	c.logger.Warn("environment size exceeds the limit", zap.Int("size", size), zap.Int("limit", MaxEnvironSizeInBytes))

	if stdoutEnvIdx == -1 {
		return errors.New("env is too large; no stdout env to trim")
	}

	stdoutCap := MaxEnvironSizeInBytes - size + len(environ[stdoutEnvIdx])
	if stdoutCap < 0 {
		return errors.New("env is too large even if trimming stdout env")
	}

	key, value := splitEnv(environ[stdoutEnvIdx])
	environ[stdoutEnvIdx] = CreateEnv(key, value[len(value)-stdoutCap:])

	return nil
}

func (c *base) getEnv(key string) string {
	env := c.Env()
	for i := len(env) - 1; i >= 0; i-- {
		e := env[i]
		prefix := key + "="
		if strings.HasPrefix(e, prefix) {
			return e[len(prefix):]
		}
	}
	return ""
}

func (c *base) lookPath(path string) (string, error) {
	pathEnv := c.getEnv("PATH")
	return c.runtime.LookPathUsingPathEnv(path, pathEnv)
}

func (c *base) findDefaultProgram(name string, args []string) (string, []string, error) {
	name, normArgs := normalizeProgramName(name)
	if IsShellLanguage(name) {
		globalShell := shellFromShellPath(c.globalShellPath())
		res, err := c.lookPath(globalShell)
		if err != nil {
			return "", nil, errors.Errorf("failed lookup default shell %s", globalShell)
		}
		return res, append(normArgs, args...), nil
	}
	// Default to "cat" for shebang++
	res, err := c.lookPath("cat")
	if err != nil {
		return "", nil, errors.Errorf("failed lookup default program cat")
	}
	return res, args, nil
}

func (c *base) findProgramInPath(name string, args []string) (string, []string, error) {
	name, normArgs := normalizeProgramName(name)
	res, err := c.lookPath(name)
	if err != nil {
		return "", nil, errors.Errorf("failed program lookup %q", name)
	}
	return res, append(normArgs, args...), nil
}

func (c *base) findProgramInKnownInterpreters(programName string, args []string) (string, []string, error) {
	interpreters := inferInterpreterFromLanguage(programName)
	if len(interpreters) == 0 {
		return "", nil, errors.Errorf("unsupported language %q", programName)
	}

	for _, interpreter := range interpreters {
		interProgram, interArgs := normalizeProgramName(interpreter)
		if path, err := c.lookPath(interProgram); err == nil {
			return path, append(interArgs, args...), nil
		}
	}

	cat, err := c.lookPath("cat")
	if err == nil {
		return cat, nil, nil
	}

	return "", nil, errors.Errorf("failed to find known interpreter out of %s", interpreters)
}

func (c *base) Stdin() io.Reader {
	return c.stdin
}

func (c *base) Stdout() io.Writer {
	if c.stdout == nil {
		c.stdout = io.Discard
	}
	return c.stdout
}

func (c *base) Stderr() io.Writer {
	if c.stderr == nil {
		c.stderr = io.Discard
	}
	return c.stderr
}

func (c *base) globalShellPath() string {
	shell := c.getEnv("SHELL")
	if shell == "" {
		shell = "sh"
	}
	if path, err := c.lookPath(shell); err == nil {
		return path
	}
	return "/bin/sh"
}

// TODO(sebastian): this method for determining shell is not strong, since shells can
// be aliased. we should probably run the shell to get this information
func shellFromShellPath(programPath string) string {
	programFile := filepath.Base(programPath)
	return programFile[:len(programFile)-len(filepath.Ext(programFile))]
}

// normalizeProgramName handles cases when the program is, for instance, "deno run".
// Only the first word is a program name and the rest is arguments.
func normalizeProgramName(name string) (_ string, args []string) {
	parts := strings.SplitN(name, " ", 2)
	if len(parts) > 0 {
		name = parts[0]
	}
	if len(parts) > 1 {
		args = strings.Split(parts[1], " ")
	}
	return name, args
}

func inferInterpreterFromLanguage(langID string) []string {
	return interpreterByLanguageID[langID]
}

var interpreterByLanguageID = map[string][]string{
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

	"lua":    {"lua"},
	"perl":   {"perl"},
	"php":    {"php"},
	"python": {"python3", "python"},
	"py":     {"python3", "python"},
	"ruby":   {"ruby"},
	"rb":     {"ruby"},
}

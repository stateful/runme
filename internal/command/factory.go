package command

import (
	"io"

	"go.uber.org/zap"

	"github.com/stateful/runme/v3/internal/dockerexec"
	"github.com/stateful/runme/v3/internal/ulid"
	runnerv2alpha1 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/runner/v2alpha1"
	"github.com/stateful/runme/v3/pkg/project"
)

type CommandOptions struct {
	// EnableEcho enables the echo when typing in the terminal.
	// It's respected only by interactive commands, i.e. composed
	// with [virtualCommand].
	EnableEcho  bool
	Session     *Session
	StdinWriter io.Writer
	Stdin       io.Reader
	Stdout      io.Writer
	Stderr      io.Writer
}

type Factory interface {
	Build(*ProgramConfig, CommandOptions) Command
}

type FactoryOption func(*commandFactory)

func WithDocker(docker *dockerexec.Docker) FactoryOption {
	return func(f *commandFactory) {
		f.docker = docker
	}
}

func WithLogger(logger *zap.Logger) FactoryOption {
	return func(f *commandFactory) {
		f.logger = logger
	}
}

func WithProject(proj *project.Project) FactoryOption {
	return func(f *commandFactory) {
		f.project = proj
	}
}

func NewFactory(opts ...FactoryOption) Factory {
	f := &commandFactory{}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

type commandFactory struct {
	docker  *dockerexec.Docker // used only for [dockerCommand]
	logger  *zap.Logger
	project *project.Project
}

// Build creates a new command based on the provided [ProgramConfig] and [CommandOptions].
//
// There are three types of commands that are :
//   - [base] - the base command that is used by all other commands. It provides
//     generic, runtime agnostic functionality. It's not fully functional, though.
//   - [nativeCommand], [virtualCommand], and [dockerCommand] - are mid-layer commands
//     built on top of the [base] command. They are fully functional, but they
//     don't really fit any real world use case. They are runtime specific.
//   - [inlineCommand], [inlineShellCommand], [terminalCommand], and [fileCommand] - are
//     high-level commands that are built on top of the mid-layer commands. They implement
//     real world use cases and are fully functional and can be used by callers.
func (f *commandFactory) Build(cfg *ProgramConfig, opts CommandOptions) Command {
	mode := cfg.Mode
	// For backward compatibility, if the mode is not specified,
	// we will try to infer it from the language. If it's shell,
	// we default it to inline.
	if mode == runnerv2alpha1.CommandMode_COMMAND_MODE_UNSPECIFIED && isShell(cfg) {
		mode = runnerv2alpha1.CommandMode_COMMAND_MODE_INLINE
	}

	// Session should be always available. If non is provided,
	// return a new one.
	if opts.Session == nil {
		opts.Session = NewSession()
	}

	switch mode {
	case runnerv2alpha1.CommandMode_COMMAND_MODE_INLINE:
		if isShell(cfg) {
			return &inlineShellCommand{
				internalCommand: f.buildInternal(cfg, opts),
				logger:          f.getLogger("InlineShellCommand"),
				session:         opts.Session,
			}
		}
		return &inlineCommand{
			internalCommand: f.buildInternal(cfg, opts),
			logger:          f.getLogger("InlineCommand"),
		}
	case runnerv2alpha1.CommandMode_COMMAND_MODE_TERMINAL:
		// For terminal commands, we always want them to be interactive.
		cfg.Interactive = true
		// And echo typed characters.
		opts.EnableEcho = true

		return &terminalCommand{
			internalCommand: f.buildVirtual(f.buildBase(cfg, opts), opts),
			logger:          f.getLogger("TerminalCommand"),
			session:         opts.Session,
			stdinWriter:     opts.StdinWriter,
		}
	case runnerv2alpha1.CommandMode_COMMAND_MODE_FILE:
		fallthrough
	default:
		return &fileCommand{
			internalCommand: f.buildInternal(cfg, opts),
			logger:          f.getLogger("FileCommand"),
		}
	}
}

func (f *commandFactory) buildBase(cfg *ProgramConfig, opts CommandOptions) *base {
	var runtime runtime
	if f.docker != nil {
		runtime = &dockerRuntime{Docker: f.docker}
	} else {
		runtime = &hostRuntime{}
	}

	return &base{
		cfg:         cfg,
		project:     f.project,
		runtime:     runtime,
		session:     opts.Session,
		stdin:       opts.Stdin,
		stdinWriter: opts.StdinWriter,
		stdout:      opts.Stdout,
		stderr:      opts.Stderr,
	}
}

func (f *commandFactory) buildInternal(cfg *ProgramConfig, opts CommandOptions) internalCommand {
	base := f.buildBase(cfg, opts)

	switch {
	case f.docker != nil:
		return f.buildDocker(base)
	case base.Interactive():
		return f.buildVirtual(base, opts)
	default:
		return f.buildNative(base)
	}
}

func (f *commandFactory) buildDocker(base *base) internalCommand {
	return &dockerCommand{
		base:   base,
		docker: f.docker,
		logger: f.getLogger("DockerCommand"),
	}
}

func (f *commandFactory) buildNative(base *base) internalCommand {
	return &nativeCommand{
		base:   base,
		logger: f.getLogger("NativeCommand"),
	}
}

func (f *commandFactory) buildVirtual(base *base, opts CommandOptions) internalCommand {
	var stdin io.ReadCloser
	if in := base.Stdin(); !isNil(in) {
		stdin = &readCloser{r: in, done: make(chan struct{})}
	}
	return &virtualCommand{
		base:          base,
		isEchoEnabled: opts.EnableEcho,
		logger:        f.getLogger("VirtualCommand"),
		stdin:         stdin,
	}
}

func (f *commandFactory) getLogger(name string) *zap.Logger {
	id := ulid.GenerateID()
	return f.logger.Named(name).With(zap.String("instanceID", id))
}

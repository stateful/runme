package command

import (
	"io"
	"reflect"

	"go.uber.org/zap"

	"github.com/stateful/runme/v3/internal/dockerexec"
	"github.com/stateful/runme/v3/internal/ulid"
	runnerv2 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/runner/v2"
	"github.com/stateful/runme/v3/pkg/project"
)

var (
	envCollectorEnableEncryption = true
	envCollectorUseFifo          = true
)

type CommandOptions struct {
	// EnableEcho enables the echo when typing in the terminal.
	// It's respected only by interactive commands, i.e. composed
	// with [virtualCommand].
	EnableEcho bool

	// Session is used to share the state between commands.
	// If none is provided, an empty one will be used.
	Session *Session

	// StdinWriter is used by [terminalCommand].
	StdinWriter io.Writer
	Stdin       io.Reader
	Stdout      io.Writer
	Stderr      io.Writer
}

type Factory interface {
	Build(*ProgramConfig, CommandOptions) (Command, error)
}

type FactoryOption func(*commandFactory)

// WithDebug enables additional debug information.
// For example, for shell commands it prints out
// commands before execution.
func WithDebug() FactoryOption {
	return func(f *commandFactory) {
		f.debug = true
	}
}

// WithDocker provides a docker client for docker commands.
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

func WithRuntime(r Runtime) FactoryOption {
	return func(f *commandFactory) {
		f.runtime = r
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
	debug   bool
	docker  *dockerexec.Docker
	logger  *zap.Logger
	project *project.Project
	runtime Runtime
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
func (f *commandFactory) Build(cfg *ProgramConfig, opts CommandOptions) (Command, error) {
	mode := cfg.Mode
	// For backward compatibility, if the mode is not specified,
	// we will try to infer it from the language. If it's shell,
	// we default it to inline.
	if mode == runnerv2.CommandMode_COMMAND_MODE_UNSPECIFIED && isShell(cfg) {
		mode = runnerv2.CommandMode_COMMAND_MODE_INLINE
	}

	// Session should be always available.
	if opts.Session == nil {
		opts.Session = NewSession()
	}

	switch mode {
	case runnerv2.CommandMode_COMMAND_MODE_INLINE:
		if isShell(cfg) {
			collector, err := f.getEnvCollector()
			if err != nil {
				return nil, err
			}

			return &inlineShellCommand{
				debug:           f.debug,
				envCollector:    collector,
				internalCommand: f.buildInternal(cfg, opts),
				logger:          f.getLogger("InlineShellCommand"),
				session:         opts.Session,
			}, nil
		}
		return &inlineCommand{
			internalCommand: f.buildInternal(cfg, opts),
			logger:          f.getLogger("InlineCommand"),
		}, nil

	case runnerv2.CommandMode_COMMAND_MODE_CLI:
		base := f.buildBase(cfg, opts)

		// In order to support interactive commands like runme-in-runme,
		// a native command is needed and creation of a new process ID
		// should be disabled.
		internal := f.buildNative(base)
		internal.disableNewProcessID = true

		if isShell(cfg) {
			collector, err := f.getEnvCollector()
			if err != nil {
				return nil, err
			}

			return &inlineShellCommand{
				debug:           f.debug,
				envCollector:    collector,
				internalCommand: internal,
				logger:          f.getLogger("InlineShellCommand"),
				session:         opts.Session,
			}, nil
		}
		return &inlineCommand{
			internalCommand: internal,
			logger:          f.getLogger("InlineCommand"),
		}, nil

	case runnerv2.CommandMode_COMMAND_MODE_TERMINAL:
		collector, err := f.getEnvCollector()
		if err != nil {
			return nil, err
		}

		// For terminal commands, we always want them to be interactive.
		cfg.Interactive = true
		// And echo typed characters.
		opts.EnableEcho = true

		return &terminalCommand{
			internalCommand: f.buildVirtual(f.buildBase(cfg, opts), opts),
			envCollector:    collector,
			logger:          f.getLogger("TerminalCommand"),
			session:         opts.Session,
			stdinWriter:     opts.StdinWriter,
		}, nil
	case runnerv2.CommandMode_COMMAND_MODE_FILE:
		fallthrough
	default:
		return &fileCommand{
			internalCommand: f.buildInternal(cfg, opts),
			logger:          f.getLogger("FileCommand"),
		}, nil
	}
}

func (f *commandFactory) buildBase(cfg *ProgramConfig, opts CommandOptions) *base {
	runtime := f.runtime

	if isNil(runtime) && f.docker != nil {
		runtime = &dockerRuntime{Docker: f.docker}
	} else if isNil(runtime) {
		runtime = &hostRuntime{useSystem: true}
	}

	return &base{
		cfg:     cfg,
		logger:  f.getLogger("Base"),
		project: f.project,
		runtime: runtime,
		session: opts.Session,
		stdin:   opts.Stdin,
		stdout:  opts.Stdout,
		stderr:  opts.Stderr,
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

func (f *commandFactory) buildDocker(base *base) *dockerCommand {
	return &dockerCommand{
		base:   base,
		docker: f.docker,
		logger: f.getLogger("DockerCommand"),
	}
}

func (f *commandFactory) buildNative(base *base) *nativeCommand {
	return &nativeCommand{
		base:   base,
		logger: f.getLogger("NativeCommand"),
	}
}

func (f *commandFactory) buildVirtual(base *base, opts CommandOptions) *virtualCommand {
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

// TODO(adamb): env collector (fifo) might need a context which will unblock it when the command finishes.
// Otherwise, it won't know when to finish waiting for the output from env producer.
func (f *commandFactory) getEnvCollector() (envCollector, error) {
	if f.docker != nil {
		return nil, nil
	}
	collectorFactory := newEnvCollectorFactory(
		envCollectorFactoryOptions{
			encryptionEnabled: envCollectorEnableEncryption,
			useFifo:           envCollectorUseFifo,
		},
	)
	return collectorFactory.Build()
}

func (f *commandFactory) getLogger(name string) *zap.Logger {
	id := ulid.GenerateID()
	return f.logger.Named(name).With(zap.String("instanceID", id))
}

func isNil(val any) bool {
	if val == nil {
		return true
	}

	v := reflect.ValueOf(val)

	switch v.Type().Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Pointer, reflect.UnsafePointer:
		return v.IsNil()
	default:
		return false
	}
}

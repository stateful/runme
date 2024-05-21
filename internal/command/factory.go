package command

import (
	"io"

	"go.uber.org/zap"

	"github.com/stateful/runme/v3/internal/config"
	"github.com/stateful/runme/v3/internal/ulid"
	runnerv2alpha1 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/runner/v2alpha1"
)

type Options struct {
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
	Build(*ProgramConfig, Options) Command
}

func NewFactory(_ *config.Config, runtime Runtime, logger *zap.Logger) Factory {
	return &commandFactory{
		runtime: runtime,
		logger:  logger,
	}
}

type commandFactory struct {
	_       *config.Config // unused yet
	runtime Runtime
	logger  *zap.Logger
}

func (f *commandFactory) Build(cfg *ProgramConfig, opts Options) Command {
	mode := cfg.Mode
	// For backward compatibility, if the mode is not specified,
	// we will try to infer it from the language. If it's shell,
	// we default it to inline.
	if mode == runnerv2alpha1.CommandMode_COMMAND_MODE_UNSPECIFIED && isShell(cfg) {
		mode = runnerv2alpha1.CommandMode_COMMAND_MODE_INLINE
	}

	switch mode {
	case runnerv2alpha1.CommandMode_COMMAND_MODE_INLINE:
		if isShell(cfg) {
			return &inlineShellCommand{
				internalCommand: f.buildInternal(cfg, opts),
				logger:          f.getLogger("InlineShellCommand"),
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
			internalCommand: f.buildVirtual(f.buildBase(cfg, opts)),
			logger:          f.getLogger("TerminalCommand"),
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

func (f *commandFactory) buildBase(cfg *ProgramConfig, opts Options) *base {
	return &base{
		cfg:           cfg,
		isEchoEnabled: opts.EnableEcho,
		runtime:       f.runtime,
		session:       opts.Session,
		stdin:         opts.Stdin,
		stdinWriter:   opts.StdinWriter,
		stdout:        opts.Stdout,
		stderr:        opts.Stderr,
	}
}

func (f *commandFactory) buildInternal(cfg *ProgramConfig, opts Options) internalCommand {
	base := f.buildBase(cfg, opts)

	if k, ok := f.runtime.(*Docker); ok {
		return &dockerCommand{
			internalCommand: base,
			docker:          k.docker,
			logger:          f.getLogger("DockerCommand"),
		}
	}

	if base.Interactive() {
		return f.buildVirtual(base)
	}

	return f.buildNative(base)
}

func (f *commandFactory) buildNative(base *base) internalCommand {
	return &nativeCommand{
		internalCommand: base,
		logger:          f.getLogger("NativeCommand"),
	}
}

func (f *commandFactory) buildVirtual(base *base) internalCommand {
	var stdin io.ReadCloser

	if !isNil(base.Stdin()) {
		stdin = &readCloser{r: base.Stdin(), done: make(chan struct{})}
		base.stdin = stdin
	}

	return &virtualCommand{
		internalCommand: base,
		stdin:           stdin,
		logger:          f.getLogger("VirtualCommand"),
	}
}

func (f *commandFactory) getLogger(name string) *zap.Logger {
	id := ulid.GenerateID()
	return f.logger.Named(name).With(zap.String("instanceID", id))
}

package command

import (
	"path/filepath"

	"github.com/stateful/runme/v3/internal/config"
	runnerv2alpha1 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/runner/v2alpha1"
)

type Factory interface {
	Build(*ProgramConfig, Options) Command
}

func NewFactory(cfg *config.Config, kernel Kernel) Factory {
	if kernel == nil {
		kernel = NewLocalKernel(nil)
	}
	return &commandFactory{cfg: cfg, kernel: kernel}
}

type commandFactory struct {
	cfg    *config.Config
	kernel Kernel
}

func (f *commandFactory) Build(cfg *ProgramConfig, opts Options) Command {
	// TODO(adamb): kernel should be a factor here too.

	switch cfg.Mode {
	case runnerv2alpha1.CommandMode_COMMAND_MODE_FILE:
		base := f.buildBaseCommand(cfg, opts)
		return newFileCommand(base)
	case runnerv2alpha1.CommandMode_COMMAND_MODE_INLINE:
		base := f.buildBaseCommand(cfg, opts)
		if isShellLanguage(cfg.LanguageId) || isShellLanguage(filepath.Base(cfg.ProgramName)) {
			return newInlineShell(base)
		}
		return newInline(base)
	case runnerv2alpha1.CommandMode_COMMAND_MODE_TERMINAL:
		return newTerminal(cfg, opts)
	default:
		return newNative(cfg, opts)
	}
}

func (f *commandFactory) buildBaseCommand(cfg *ProgramConfig, opts Options) internalCommand {
	if k, ok := f.kernel.(*DockerKernel); ok {
		return newDocker(k.docker, cfg, opts)
	}
	if cfg.Interactive {
		return newVirtual(cfg, opts)
	}
	return newNative(cfg, opts)
}

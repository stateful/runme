package runner

import (
	"context"
	"io"

	"go.uber.org/zap"

	"github.com/stateful/runme/v3/internal/executable"
)

type Executable interface {
	DryRun(context.Context, io.Writer)
	Run(context.Context) error
	ExitCode() int
}

type ExecutableConfig struct {
	Name    string
	Dir     string
	Tty     bool
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
	PreEnv  []string
	PostEnv []string
	Session *Session
	Logger  *zap.Logger
}

func IsSupported(lang string) bool {
	return executable.IsSupported(lang)
}

func IsShell(lang string) bool {
	return executable.IsShell(lang)
}

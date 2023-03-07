package runner

import (
	"context"
	"io"

	"go.uber.org/zap"
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
	Session *Session
	Logger  *zap.Logger
}

var supportedExecutables = []string{
	"bash",
	"bat", // fallback to sh
	"sh",
	"sh-raw",
	"shell",
	"zsh",
	"go",
}

func IsSupported(lang string) bool {
	for _, item := range supportedExecutables {
		if item == lang {
			return true
		}
	}
	return false
}

func IsShell(lang string) bool {
	return lang == "sh" || lang == "shell" || lang == "sh-raw"
}

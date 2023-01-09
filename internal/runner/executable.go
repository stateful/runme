package runner

import (
	"context"
	"io"

	"github.com/stateful/runme/internal/document"
)

type Executable interface {
	DryRun(context.Context, io.Writer)
	Run(context.Context) error
}

type Base struct {
	Dir    string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
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

func IsShell(block *document.CodeBlock) bool {
	lang := block.Language()
	return lang == "sh" || lang == "shell" || lang == "sh-raw"
}

package runner

import (
	"context"
	"io"
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
	exec := block.Executable()
	return exec == "sh" || exec == "shell" || exec == "sh-raw"
}

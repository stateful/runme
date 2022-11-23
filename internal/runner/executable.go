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

var supportedExecutables = map[string]struct{}{"sh": {}, "shell": {}, "sh-raw": {}, "go": {}}

func IsSupported(lang string) bool {
	_, ok := supportedExecutables[lang]
	return ok
}

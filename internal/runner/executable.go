package runner

import (
	"context"
	"io"
)

type Executable interface {
	Run(context.Context) error
}

type Base struct {
	Dir    string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

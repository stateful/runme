// Package runner defining an interface defining for an executable
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

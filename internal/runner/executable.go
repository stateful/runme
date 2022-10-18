package runner

import (
	"context"
	"io"

	"github.com/pkg/errors"
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

func New(block *document.CodeBlock, base *Base) (Executable, error) {
	switch block.Executable() {
	case "sh", "shell":
		return &Shell{
			Cmds: block.Lines(),
			Base: base,
		}, nil
	case "go":
		return &Go{
			Source: block.Content(),
			Base:   base,
		}, nil
	default:
		return nil, errors.Errorf("unknown executable: %q", block.Executable())
	}
}

var supportedExecutables = map[string]struct{}{"sh": {}, "shell": {}, "go": {}}

func IsSupported(block *document.CodeBlock) bool {
	_, ok := supportedExecutables[block.Executable()]
	return ok
}

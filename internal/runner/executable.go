package runner

import (
	"context"
	"io"
	"strings"

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
	case "sh-raw":
		return &ShellRaw{
			Cmds: block.Lines(),
			Base: base,
		}, nil
	case "go":
		lines := strings.Split(block.Content(), "\n")
		if len(lines) < 2 {
			return nil, errors.New("invalid content for \"go\" executable")
		}
		return &Go{
			Source: strings.Join(lines[1:len(lines)-1], "\n"),
			Base:   base,
		}, nil
	default:
		return nil, errors.Errorf("unknown executable: %q", block.Executable())
	}
}

var supportedExecutables = map[string]struct{}{"sh": {}, "shell": {}, "sh-raw": {}, "go": {}}

func IsSupported(block *document.CodeBlock) bool {
	_, ok := supportedExecutables[block.Executable()]
	return ok
}

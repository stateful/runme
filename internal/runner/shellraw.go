package runner

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

type ShellRaw struct {
	*Base
	Cmds []string
}

var _ Executable = (*Shell)(nil)

func (s *ShellRaw) DryRun(ctx context.Context, w io.Writer) {
	sh, ok := os.LookupEnv("SHELL")
	if !ok {
		sh = "/bin/sh"
	}

	var b strings.Builder

	_, _ = b.WriteString(fmt.Sprintf("#!%s\n\n", sh))
	_, _ = b.WriteString(fmt.Sprintf("// run in %q\n\n", s.Dir))
	_, _ = b.WriteString(strings.Join(s.Cmds, "\n"))

	_, err := w.Write([]byte(b.String()))
	if err != nil {
		log.Fatalf("failed to write: %s", err)
	}
}

func (s *ShellRaw) Run(ctx context.Context) error {
	sh, ok := os.LookupEnv("SHELL")
	if !ok {
		sh = "/bin/sh"
	}
	return execSingle(ctx, sh, s.Dir, s.Session, nil, strings.Join(s.Cmds, "\n"), s.Name, s.Stdin, s.Stdout, s.Stderr)
}

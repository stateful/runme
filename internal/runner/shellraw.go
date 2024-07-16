package runner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"strings"
)

type ShellRaw struct {
	*Shell
}

var _ Executable = (*ShellRaw)(nil)

func (s ShellRaw) DryRun(ctx context.Context, w io.Writer) {
	var b bytes.Buffer

	_, _ = b.WriteString(fmt.Sprintf("#!%s\n\n", s.ProgramPath()))
	_, _ = b.WriteString(fmt.Sprintf("// run in %q\n\n", s.Dir))
	_, _ = b.WriteString(prepareScript(strings.Join(s.Cmds, "\n"), s.ShellType()))

	_, err := w.Write(b.Bytes())
	if err != nil {
		log.Fatalf("failed to write: %s", err)
	}
}

func (s ShellRaw) Run(ctx context.Context) error {
	cmd, err := newCommand(
		ctx,
		&commandConfig{
			ProgramName: s.ProgramPath(),
			Directory:   s.Dir,
			Session:     s.Session,
			Tty:         s.Tty,
			Stdin:       s.Stdin,
			Stdout:      s.Stdout,
			Stderr:      s.Stderr,
			CommandMode: CommandModeInlineShell,
			Commands:    nil,
			Script:      strings.Join(s.Cmds, "\n"),
			Logger:      s.Logger,
		},
	)
	if err != nil {
		return err
	}
	return s.run(ctx, cmd)
}

package runner

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
)

type Shell struct {
	Base
	Cmds []string
}

var _ Executable = (*Shell)(nil)

func (s Shell) DryRun(ctx context.Context, w io.Writer) {
	sh, ok := os.LookupEnv("SHELL")
	if !ok {
		sh = "/bin/sh"
	}

	var b strings.Builder

	_, _ = b.WriteString(fmt.Sprintf("#!%s\n\n", sh))
	_, _ = b.WriteString(fmt.Sprintf("// run in %q\n\n", s.Dir))
	_, _ = b.WriteString(s.prepareScript())

	_, err := w.Write([]byte(b.String()))
	if err != nil {
		log.Fatalf("failed to write: %s", err)
	}
}

func (s Shell) Run(ctx context.Context) error {
	sh, ok := os.LookupEnv("SHELL")
	if !ok {
		sh = "/bin/sh"
	}

	return execSingle(ctx, sh, s.Dir, s.prepareScript(), s.Stdin, s.Stdout, s.Stderr)
}

func (s Shell) prepareScript() string {
	var b strings.Builder

	_, _ = b.WriteString("set -e -o pipefail;")

	for _, cmd := range s.Cmds {
		_, _ = b.WriteString(fmt.Sprintf("%s;", cmd))
	}

	_, _ = b.WriteRune('\n')

	return b.String()
}

func execSingle(ctx context.Context, sh, dir, cmd string, stdin io.Reader, stdout, stderr io.Writer) error {
	c := exec.CommandContext(ctx, sh, []string{"-c", cmd}...)
	c.Dir = dir
	c.Stderr = stderr
	c.Stdout = stdout
	c.Stdin = stdin

	return errors.Wrapf(c.Run(), "failed to run command %q", cmd)
}

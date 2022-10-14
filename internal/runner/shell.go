package runner

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/google/shlex"
	"github.com/pkg/errors"
)

type Shell struct {
	Base
	Cmds []string
}

var _ Executable = (*Shell)(nil)

func (s *Shell) DryRun(ctx context.Context, w io.Writer) {
	sh, ok := os.LookupEnv("SHELL")
	if !ok {
		sh = "/bin/sh"
	}

	var b strings.Builder

	_, _ = b.WriteString(fmt.Sprintf("#!%s\n\n", sh))
	_, _ = b.WriteString(fmt.Sprintf("// run in %q\n\n", s.Dir))
	_, _ = b.WriteString(prepareScript(s.Cmds))

	_, err := w.Write([]byte(b.String()))
	if err != nil {
		log.Fatalf("failed to write: %s", err)
	}
}

func (s *Shell) Run(ctx context.Context) error {
	sh, ok := os.LookupEnv("SHELL")
	if !ok {
		sh = "/bin/sh"
	}

	return execSingle(ctx, sh, s.Dir, prepareScript(s.Cmds), s.Stdin, s.Stdout, s.Stderr)
}

func prepareScript(cmds []string) string {
	var b strings.Builder

	_, _ = b.WriteString("set -e -o pipefail;")

	for _, cmd := range cmds {
		detectedWord := false
		lex := shlex.NewLexer(strings.NewReader(cmd))

		for {
			word, err := lex.Next()
			if err != nil {
				if err.Error() == "EOF found after escape character" {
					// Handle the case when a line ends with "\"
					// which should continue a single command.
					_, _ = b.WriteString(" ")
				} else if detectedWord {
					_, _ = b.WriteString(";")
				}
				break
			}

			if detectedWord {
				// Separate words with a space. It's done in this way
				// to avoid trailing spaces.
				_, _ = b.WriteString(" ")
			} else {
				detectedWord = true
			}

			_, _ = b.WriteString(word)
		}
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

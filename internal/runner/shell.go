package runner

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/google/shlex"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type Shell struct {
	*Base
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
	_, _ = b.WriteString(prepareScriptFromCommands(s.Cmds))

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
	return execSingle(ctx, sh, s.Dir, s.Cmds, "", s.Name, s.Stdin, s.Stdout, s.Stderr)
}

func PrepareScriptFromCommands(cmds []string) string {
	return prepareScriptFromCommands(cmds)
}

func prepareScriptFromCommands(cmds []string) string {
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

			if strings.Contains(word, " ") {
				_, _ = b.WriteString(strconv.Quote(word))
			} else {
				_, _ = b.WriteString(word)
			}
		}
	}

	_, _ = b.WriteRune('\n')

	return b.String()
}

func prepareScript(script string) string {
	var b strings.Builder

	_, _ = b.WriteString("set -e -o pipefail;")
	_, _ = b.WriteString(script)
	_, _ = b.WriteRune('\n')

	return b.String()
}

func execSingle(
	ctx context.Context,
	sh string,
	dir string,
	commands []string,
	script string,
	name string,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
) error {
	cmd, err := newCommand(
		&commandConfig{
			ProgramName: sh,
			Directory:   dir,
			Tty:         true,
			IsShell:     true,
			Commands:    commands,
			Script:      script,
		},
		zap.NewNop(),
	)
	if err != nil {
		return err
	}

	errc := make(chan error)
	go func() {
		_, err := io.Copy(cmd.Stdin, stdin)
		if errors.Is(err, os.ErrClosed) {
			err = nil
		}
		errc <- err
	}()

	_, err = executeCmd(
		ctx,
		cmd,
		&startOpts{DisableEcho: true},
		func(o output) error {
			if _, err := stdout.Write(o.Stdout); err != nil {
				return errors.WithStack(err)
			}
			if _, err := stdout.Write(o.Stderr); err != nil {
				return errors.WithStack(err)
			}
			return nil
		},
	)
	if err != nil {
		var exiterr *exec.ExitError
		// Ignore errors caused by SIGINT.
		if errors.As(err, &exiterr) && exiterr.ProcessState.Sys().(syscall.WaitStatus).Signal() != os.Kill {
			msg := "failed to run command"
			if len(name) > 0 {
				msg += " " + strconv.Quote(name)
			}
			return errors.Wrap(err, msg)
		}
	}
	select {
	case err := <-errc:
		return errors.Wrap(err, "failed to copy stdin")
	default:
		return nil
	}
}

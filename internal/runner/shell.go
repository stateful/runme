package runner

import (
	"bytes"
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
)

type Shell struct {
	*ExecutableConfig
	command *command
	Cmds    []string
}

var _ Executable = (*Shell)(nil)

func (s Shell) ProgramPath() string {
	return ShellPath()
}

func (s Shell) DryRun(ctx context.Context, w io.Writer) {
	var b bytes.Buffer

	_, _ = b.WriteString(fmt.Sprintf("#!%s\n\n", s.ProgramPath()))
	_, _ = b.WriteString(fmt.Sprintf("// run in %q\n\n", s.Dir))
	_, _ = b.WriteString(prepareScriptFromCommands(s.Cmds))

	_, err := w.Write(b.Bytes())
	if err != nil {
		log.Fatalf("failed to write: %s", err)
	}
}

func (s *Shell) Run(ctx context.Context) error {
	cmd, err := newCommand(
		&commandConfig{
			ProgramName: s.ProgramPath(),
			Directory:   s.Dir,
			Session:     s.Session,
			Tty:         s.Tty,
			Stdin:       s.Stdin,
			Stdout:      s.Stdout,
			Stderr:      s.Stderr,
			IsShell:     true,
			Commands:    s.Cmds,
			Script:      "",
			Logger:      s.Logger,
		},
	)
	if err != nil {
		return err
	}
	s.command = cmd
	return s.run(ctx, cmd)
}

func (s Shell) ExitCode() int {
	if s.command == nil || s.command.cmd == nil {
		return -1
	}

	return s.command.cmd.ProcessState.ExitCode()
}

func (s Shell) run(ctx context.Context, cmd *command) error {
	opts := &startOpts{}
	if s.Tty {
		opts.DisableEcho = true
	}

	if err := cmd.StartWithOpts(ctx, opts); err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		var exiterr *exec.ExitError
		// Ignore errors caused by SIGINT.
		if errors.As(err, &exiterr) {
			var rerr error = ExitErrorFromExec(exiterr)

			if exiterr.ProcessState.Sys().(syscall.WaitStatus).Signal() != os.Kill {
				msg := "failed to run command"
				if len(s.Name) > 0 {
					msg += " " + strconv.Quote(s.Name)
				}
				return errors.Wrap(rerr, msg)
			}

			return rerr
		}
	}

	return nil
}

func ShellPath() string {
	shell, ok := os.LookupEnv("SHELL")
	if !ok {
		shell = "sh"
	}
	if path, err := exec.LookPath(shell); err == nil {
		return path
	}
	return "/bin/sh"
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

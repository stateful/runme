package runner

import (
	"bytes"
	"context"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"github.com/pkg/errors"
)

type DaggerShell struct {
	*ExecutableConfig
	command     *command
	Script      string
	ProgramName string
	LanguageID  string
}

var _ Executable = (*DaggerShell)(nil)

func (s DaggerShell) DryRun(ctx context.Context, w io.Writer) {
	var b bytes.Buffer

	_, err := w.Write(b.Bytes())
	if err != nil {
		log.Fatalf("failed to write: %s", err)
	}
}

func (s *DaggerShell) Run(ctx context.Context) error {
	cmd, err := newCommand(
		ctx,
		&commandConfig{
			ProgramName: s.ProgramName,
			LanguageID:  s.LanguageID,
			Directory:   s.Dir,
			Session:     s.Session,
			Tty:         s.Tty,
			Stdin:       s.Stdin,
			Stdout:      s.Stdout,
			Stderr:      s.Stderr,
			CommandMode: CommandModeDaggerShell,
			Script:      s.Script,
			Logger:      s.Logger,
		},
	)
	if err != nil {
		return err
	}
	s.command = cmd
	defer func() { _ = s.command.Finalize() }()
	return s.run(ctx, cmd)
}

func (s DaggerShell) ExitCode() int {
	if s.command == nil || s.command.cmd == nil {
		return -1
	}

	return s.command.cmd.ProcessState.ExitCode()
}

func (s DaggerShell) run(ctx context.Context, cmd *command) error {
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

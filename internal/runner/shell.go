package runner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/pkg/errors"

	"github.com/stateful/runme/v3/internal/system"
	"github.com/stateful/runme/v3/pkg/document"
)

type Shell struct {
	*ExecutableConfig
	command     *command
	Cmds        []string
	CustomShell string
}

var _ Executable = (*Shell)(nil)

func (s Shell) ProgramPath() string {
	return resolveShellPath(s.CustomShell)
}

func (s Shell) ShellType() string {
	return ShellFromShellPath(s.ProgramPath())
}

func (s Shell) DryRun(ctx context.Context, w io.Writer) {
	var b bytes.Buffer

	_, _ = b.WriteString(fmt.Sprintf("#!%s\n\n", s.ProgramPath()))
	_, _ = b.WriteString(fmt.Sprintf("// run in %q\n\n", s.Dir))
	_, _ = b.WriteString(prepareScriptFromCommands(s.Cmds, s.ShellType()))

	_, err := w.Write(b.Bytes())
	if err != nil {
		log.Fatalf("failed to write: %s", err)
	}
}

func (s *Shell) Run(ctx context.Context) error {
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

func IsShellLanguage(languageID string) bool {
	switch strings.ToLower(languageID) {
	// shellscripts
	case "sh", "bash", "zsh", "ksh", "shell", "shellscript":
		return true

	// dos
	case "bat", "cmd":
		return true

	// powershell
	case "powershell", "pwsh":
		return true

	// fish
	case "fish":
		return true

	default:
		return false
	}
}

func GetCellProgram(languageID string, customShell string, cell *document.CodeBlock) (program string, commandMode CommandMode) {
	if IsShellLanguage(languageID) {
		program = customShell
		commandMode = CommandModeInlineShell
	} else {
		commandMode = CommandModeTempFile
	}

	if interpreter := cell.Interpreter(); interpreter != "" {
		program = interpreter
	}

	return
}

func resolveShellPath(customShell string) string {
	if customShell != "" {
		if path, err := system.LookPath(customShell); err == nil {
			return path
		}
	}

	return globalShellPath()
}

func globalShellPath() string {
	shell, ok := os.LookupEnv("SHELL")
	if !ok {
		shell = "sh"
	}
	if path, err := system.LookPath(shell); err == nil {
		return path
	}
	return "/bin/sh"
}

// TODO(mxs): this method for determining shell is not strong, since shells can
// be aliased. we should probably run the shell to get this information
func ShellFromShellPath(programPath string) string {
	programFile := filepath.Base(programPath)
	return programFile[:len(programFile)-len(filepath.Ext(programFile))]
}

func PrepareScriptFromCommands(cmds []string, shell string) string {
	return prepareScriptFromCommands(cmds, shell)
}

func prepareScriptFromCommands(cmds []string, shell string) string {
	var b strings.Builder

	_, _ = b.WriteString(getShellOptions(shell))

	for _, cmd := range cmds {
		_, _ = b.WriteString(cmd)
		_, _ = b.WriteRune('\n')
	}

	_, _ = b.WriteRune('\n')

	return b.String()
}

func prepareScript(script string, shell string) string {
	var b strings.Builder

	_, _ = b.WriteString(getShellOptions(shell))

	_, _ = b.WriteString(script)
	_, _ = b.WriteRune('\n')

	return b.String()
}

func getShellOptions(shell string) (res string) {
	// TODO(mxs): powershell, DOS
	switch shell {
	case "zsh", "ksh", "bash":
		res += "set -e -o pipefail"
	case "sh":
		res += "set -e"
	}

	res += "\n"
	return
}

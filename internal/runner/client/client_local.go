package client

import (
	"context"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/muesli/cancelreader"
	"github.com/pkg/errors"
	"github.com/stateful/runme/internal/document"
	"github.com/stateful/runme/internal/project"
	"github.com/stateful/runme/internal/runner"
	"go.uber.org/zap"
)

type LocalRunner struct {
	*RunnerSettings

	shellID int
}

func (r *LocalRunner) Clone() Runner {
	return &LocalRunner{
		RunnerSettings: r.RunnerSettings.Clone(),
		shellID:        r.shellID,
	}
}

func (r *LocalRunner) getSettings() *RunnerSettings {
	return r.RunnerSettings
}

func (r *LocalRunner) setSettings(rs *RunnerSettings) {
	r.RunnerSettings = rs
}

func NewLocalRunner(opts ...RunnerOption) (*LocalRunner, error) {
	r := &LocalRunner{
		RunnerSettings: &RunnerSettings{},
	}

	if err := ApplyOptions(r, opts...); err != nil {
		return nil, err
	}

	if r.logger == nil {
		r.logger = zap.NewNop()
	}

	if r.withinShellMaybe {
		if id, ok := shellID(); ok {
			r.shellID = id
		}
	}

	envs := append(os.Environ(), r.envs...)

	sess, err := runner.NewSession(envs, r.logger)
	if err != nil {
		return nil, err
	}

	r.session = sess

	return r, nil
}

func (r *LocalRunner) newExecutable(task project.Task) (runner.Executable, error) {
	block := task.CodeBlock
	fmtr, err := task.CodeBlock.Document().Frontmatter()
	if err != nil {
		return nil, err
	}

	customShell := r.customShell
	if fmtr != nil && fmtr.Shell != "" {
		customShell = fmtr.Shell
	}

	programName, _ := runner.GetCellProgram(block.Language(), customShell, block)

	r.session.AddEnvs(r.envs)

	cfg := &runner.ExecutableConfig{
		Name:    block.Name(),
		Dir:     r.dir,
		Tty:     block.Interactive(),
		Stdout:  r.stdout,
		Stderr:  r.stderr,
		Session: r.session,
		Logger:  r.logger,
	}

	// TODO(adamb): what about `r.envs`?
	cfg.PreEnv, err = r.project.LoadEnvs()
	if err != nil {
		return nil, err
	}

	cfg.Dir = ResolveDirectory(cfg.Dir, task)

	if block.Interactive() {
		cfg.Stdin = r.stdin
	}

	switch block.Language() {
	// TODO(mxs): empty string should return nil when guesslang model is implemented
	case "bash", "bat", "sh", "shell", "zsh", "":
		return &runner.Shell{
			ExecutableConfig: cfg,
			Cmds:             block.Lines(),
			CustomShell:      customShell,
		}, nil
	case "sh-raw":
		return &runner.ShellRaw{
			Shell: &runner.Shell{
				ExecutableConfig: cfg,
				Cmds:             block.Lines(),
			},
		}, nil
	case "go":
		return &runner.Go{
			ExecutableConfig: cfg,
			Source:           string(block.Content()),
		}, nil
	default:
		return &runner.TempFile{
			ExecutableConfig: cfg,
			Script:           strings.Join(block.Lines(), "\n"),
			ProgramName:      programName,
			LanguageID:       block.Language(),
		}, nil
	}
}

func (r *LocalRunner) RunTask(ctx context.Context, task project.Task) error {
	block := task.CodeBlock

	if r.shellID > 0 {
		return r.runBlockInShell(ctx, block)
	}

	executable, err := r.newExecutable(task)
	if err != nil {
		return err
	}

	if executable == nil {
		return errors.Errorf("unknown executable: %q", block.Language())
	}

	// poll for exit
	// TODO(mxs): we probably want to use `StdinPipe` eventually
	if block.Interactive() {
		go func() {
			for {
				if executable.ExitCode() > -1 {
					if canceler, ok := r.stdin.(cancelreader.CancelReader); ok {
						_ = canceler.Cancel()
					}

					return
				}

				time.Sleep(100 * time.Millisecond)
			}
		}()
	}

	return errors.WithStack(executable.Run(ctx))
}

func (r *LocalRunner) runBlockInShell(ctx context.Context, block *document.CodeBlock) error {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", "/tmp/runme-"+strconv.Itoa(r.shellID)+".sock")
	if err != nil {
		return errors.WithStack(err)
	}
	for _, line := range block.Lines() {
		line = strings.TrimSpace(line)

		if _, err := conn.Write([]byte(line)); err != nil {
			return errors.WithStack(err)
		}
		if _, err := conn.Write([]byte("\n")); err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
}

func (r *LocalRunner) DryRunTask(ctx context.Context, task project.Task, w io.Writer, opts ...RunnerOption) error {
	executable, err := r.newExecutable(task)
	if err != nil {
		return err
	}

	executable.DryRun(ctx, w)

	return nil
}

func (r *LocalRunner) Cleanup(ctx context.Context) error {
	return nil
}

func shellID() (int, bool) {
	id := os.Getenv("RUNMESHELL")
	if id == "" {
		return 0, false
	}
	i, err := strconv.Atoi(id)
	if err != nil {
		return -1, false
	}
	return i, true
}

func (r *LocalRunner) GetEnvs(ctx context.Context) ([]string, error) {
	return r.session.Envs(), nil
}

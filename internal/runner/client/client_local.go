package client

import (
	"context"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/stateful/runme/internal/document"
	"github.com/stateful/runme/internal/env"
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

	r.session = runner.NewSession(os.Environ(), r.logger)

	return r, nil
}

func (r *LocalRunner) newExecutable(fileBlock project.FileCodeBlock) (runner.Executable, error) {
	block := fileBlock.GetBlock()
	fmtr := fileBlock.GetFrontmatter()

	customShell := r.customShell
	if fmtr.Shell != "" {
		customShell = fmtr.Shell
	}

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

	if r.project != nil {
		projEnvs, err := r.project.LoadEnvs()
		if err != nil {
			return nil, err
		}

		cfg.PreEnv = env.ConvertMapEnv(projEnvs)
	}

	cfg.Dir = ResolveDirectory(cfg.Dir, fileBlock)

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
		return nil, nil
	}
}

func (r *LocalRunner) RunBlock(ctx context.Context, fileBlock project.FileCodeBlock) error {
	block := fileBlock.GetBlock()

	if r.shellID > 0 {
		return r.runBlockInShell(ctx, block)
	}

	executable, err := r.newExecutable(fileBlock)
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
					if closer, ok := r.stdin.(io.ReadCloser); ok {
						_ = closer.Close()
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

func (r *LocalRunner) DryRunBlock(ctx context.Context, fileBlock project.FileCodeBlock, w io.Writer, opts ...RunnerOption) error {
	block := fileBlock.GetBlock()

	executable, err := r.newExecutable(block)
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

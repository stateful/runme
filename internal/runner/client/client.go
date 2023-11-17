package client

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/muesli/cancelreader"
	"github.com/stateful/runme/internal/document"
	runnerv1 "github.com/stateful/runme/internal/gen/proto/go/runme/runner/v1"
	"github.com/stateful/runme/internal/project"
	"github.com/stateful/runme/internal/runner"
	"go.uber.org/zap"
)

type RunnerOption func(Runner) error

var ErrRunnerClientUnimplemented = fmt.Errorf("method unimplemented")

type RunnerSettings struct {
	session         *runner.Session
	sessionID       string
	project         *project.Project
	cleanupSession  bool
	sessionStrategy runnerv1.SessionStrategy

	withinShellMaybe bool
	customShell      string

	dir string

	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer

	logger           *zap.Logger
	enableBackground bool

	insecure bool
	tlsDir   string

	envs []string
}

func (rs *RunnerSettings) Clone() *RunnerSettings {
	newRs := *rs
	return &newRs
}

type Runner interface {
	RunTask(ctx context.Context, task project.Task) error
	DryRunTask(ctx context.Context, task project.Task, w io.Writer, opts ...RunnerOption) error
	Cleanup(ctx context.Context) error

	Clone() Runner

	GetEnvs(ctx context.Context) ([]string, error)

	getSettings() *RunnerSettings
	setSettings(settings *RunnerSettings)
}

func withSettings(applySettings func(settings *RunnerSettings)) RunnerOption {
	return withSettingsErr(func(settings *RunnerSettings) error {
		applySettings(settings)
		return nil
	})
}

func withSettingsErr(applySettings func(settings *RunnerSettings) error) RunnerOption {
	return func(r Runner) error {
		return applySettings(r.getSettings())
	}
}

func WithSession(s *runner.Session) RunnerOption {
	return withSettings(func(rs *RunnerSettings) {
		rs.session = s
	})
}

func WithSessionID(id string) RunnerOption {
	return withSettings(func(rs *RunnerSettings) {
		rs.sessionID = id
	})
}

func WithProject(proj *project.Project) RunnerOption {
	return withSettings(func(rs *RunnerSettings) {
		rs.project = proj
	})
}

func WithCleanupSession(cleanup bool) RunnerOption {
	return withSettings(func(rs *RunnerSettings) {
		rs.cleanupSession = cleanup
	})
}

func WithSessionStrategy(strategy runnerv1.SessionStrategy) RunnerOption {
	return withSettings(func(rs *RunnerSettings) {
		rs.sessionStrategy = strategy
	})
}

func WithinShellMaybe() RunnerOption {
	return withSettings(func(rs *RunnerSettings) {
		rs.withinShellMaybe = true
	})
}

func WithDir(dir string) RunnerOption {
	return withSettings(func(rs *RunnerSettings) {
		rs.dir = dir
	})
}

func WithStdin(stdin io.Reader) RunnerOption {
	return withSettings(func(rs *RunnerSettings) {
		rs.stdin = stdin
	})
}

func WithStdout(stdout io.Writer) RunnerOption {
	return withSettings(func(rs *RunnerSettings) {
		rs.stdout = stdout
	})
}

func WithStderr(stderr io.Writer) RunnerOption {
	return withSettings(func(rs *RunnerSettings) {
		rs.stderr = stderr
	})
}

func WithStdinTransform(op func(io.Reader) (io.Reader, error)) RunnerOption {
	return withSettingsErr(func(rs *RunnerSettings) error {
		stdin, err := op(rs.stdin)
		if err != nil {
			return err
		}

		rs.stdin = stdin
		return nil
	})
}

func WithStdoutTransform(op func(io.Writer) io.Writer) RunnerOption {
	return withSettings(func(rs *RunnerSettings) {
		rs.stdout = op(rs.stdout)
	})
}

func WithStderrTransform(op func(io.Writer) io.Writer) RunnerOption {
	return withSettings(func(rs *RunnerSettings) {
		rs.stderr = op(rs.stderr)
	})
}

func WithLogger(logger *zap.Logger) RunnerOption {
	return withSettings(func(rs *RunnerSettings) {
		rs.logger = logger
	})
}

func WithInsecure(insecure bool) RunnerOption {
	return withSettings(func(rs *RunnerSettings) {
		rs.insecure = insecure
	})
}

func WithTLSDir(tlsDir string) RunnerOption {
	return withSettings(func(rs *RunnerSettings) {
		rs.tlsDir = tlsDir
	})
}

func WithEnableBackgroundProcesses(enableBackground bool) RunnerOption {
	return withSettings(func(rs *RunnerSettings) {
		rs.enableBackground = enableBackground
	})
}

func WithEnvs(envs []string) RunnerOption {
	return withSettings(func(rs *RunnerSettings) {
		rs.envs = envs
	})
}

func WithCustomShell(customShell string) RunnerOption {
	return withSettings(func(rs *RunnerSettings) {
		rs.customShell = customShell
	})
}

func WithTempSettings(rc Runner, opts []RunnerOption, cb func() error) error {
	oldSettings := rc.getSettings().Clone()

	err := ApplyOptions(rc, opts...)
	if err != nil {
		return err
	}

	err = cb()
	rc.setSettings(oldSettings)
	if err != nil {
		return err
	}

	return nil
}

func ApplyOptions(rc Runner, opts ...RunnerOption) error {
	for _, opt := range opts {
		if err := opt(rc); err != nil {
			return err
		}
	}

	return nil
}

func WrapWithCancelReader() RunnerOption {
	return WithStdinTransform(func(r io.Reader) (io.Reader, error) {
		return cancelreader.NewReader(r)
	})
}

func ResolveDirectory(parentDir string, task project.Task) string {
	// TODO(adamb): consider handling this error or add a comment it can be skipped.
	fmtr, _ := task.CodeBlock.Document().Frontmatter()
	if fmtr == nil {
		fmtr = &document.Frontmatter{}
	}

	dirs := []string{
		filepath.Dir(task.DocumentPath),
		filepath.FromSlash(fmtr.Cwd),
		filepath.FromSlash(task.CodeBlock.Cwd()),
	}

	for _, dir := range dirs {
		newDir := resolveOrAbsolute(parentDir, dir)

		if stat, err := osfs.Default.Stat(newDir); err == nil && stat.IsDir() {
			parentDir = newDir
		}
	}

	return parentDir
}

func resolveOrAbsolute(parent string, child string) string {
	if child == "" {
		return parent
	}

	if filepath.IsAbs(child) {
		return child
	}

	if parent != "" {
		return filepath.Join(parent, child)
	}

	return child
}

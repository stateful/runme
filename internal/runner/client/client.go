package client

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/go-git/go-billy/v5/osfs"
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
	project         project.Project
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
}

func (rs *RunnerSettings) Clone() *RunnerSettings {
	newRs := *rs
	return &newRs
}

type Runner interface {
	RunBlock(ctx context.Context, block project.FileCodeBlock) error
	DryRunBlock(ctx context.Context, block project.FileCodeBlock, w io.Writer, opts ...RunnerOption) error
	Cleanup(ctx context.Context) error

	Clone() Runner

	getSettings() *RunnerSettings
	setSettings(settings *RunnerSettings)
}

func withSettings(applySettings func(settings *RunnerSettings)) RunnerOption {
	return func(r Runner) error {
		applySettings(r.getSettings())
		return nil
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

func WithProject(proj project.Project) RunnerOption {
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

func WithStdinTransform(op func(io.Reader) io.Reader) RunnerOption {
	return withSettings(func(rs *RunnerSettings) {
		rs.stdin = op(rs.stdin)
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

func WithCustomShell(customShell string) RunnerOption {
	return withSettings(func(rs *RunnerSettings) {
		rs.customShell = customShell
	})
}

func WithTempSettings(rc Runner, opts []RunnerOption, cb func()) error {
	oldSettings := rc.getSettings().Clone()

	err := ApplyOptions(rc, opts...)
	if err != nil {
		return err
	}

	cb()

	rc.setSettings(oldSettings)

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

func ResolveDirectory(parentDir string, fileBlock project.FileCodeBlock) string {
	for _, dir := range []string{
		filepath.Dir(fileBlock.GetFile()),
		filepath.FromSlash(fileBlock.GetFrontmatter().Cwd),
		filepath.FromSlash(fileBlock.GetBlock().Cwd()),
	} {
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

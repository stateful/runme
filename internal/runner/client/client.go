package client

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/muesli/cancelreader"
	"github.com/pkg/errors"
	"github.com/stateful/runme/v3/internal/runner"
	runnerv1 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/runner/v1"
	"github.com/stateful/runme/v3/pkg/document"
	"github.com/stateful/runme/v3/pkg/project"
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

	envStoreType runnerv1.SessionEnvStoreType
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

	ResolveProgram(ctx context.Context, mode runnerv1.ResolveProgramRequest_Mode, script string, language string) (*runnerv1.ResolveProgramResponse, error)

	getSettings() *RunnerSettings
	setSettings(settings *RunnerSettings)
}

func New(context context.Context, serverAddr string, fallbackRunner bool, runnerOpts []RunnerOption) (Runner, error) {
	if serverAddr != "" {
		// check if serverAddr points at a healthy server
		healthy, err := isServerHealthy(context, serverAddr, runnerOpts)
		if err != nil {
			return nil, errors.Wrap(err, "failed to check health")
		}

		if !fallbackRunner || healthy {
			remoteRunner, err := NewRemoteRunner(
				context,
				serverAddr,
				runnerOpts...,
			)
			if err != nil {
				return nil, errors.Wrap(err, "failed to create remote runner")
			}

			return remoteRunner, nil
		}
	}

	localRunner, err := NewLocalRunner(runnerOpts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create local runner")
	}

	return localRunner, nil
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
	return withSettingsErr(func(rs *RunnerSettings) error {
		rs.project = proj

		projEnvs, err := rs.project.LoadEnv()
		rs.envs = append(rs.envs, projEnvs...)

		return errors.Wrap(err, "failed to apply project envs")
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
		rs.envs = append(rs.envs, envs...)
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

func WithEnvStoreType(EnvStoreType runnerv1.SessionEnvStoreType) RunnerOption {
	return withSettings(func(rs *RunnerSettings) {
		rs.envStoreType = EnvStoreType
	})
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
	doc := task.CodeBlock.Document()
	fmtr := doc.Frontmatter()
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

func prepareCommandSeq(script string, language string) string {
	if !runner.IsShellLanguage(language) {
		return script
	}

	lines := strings.Split(script, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "$") {
			lines[i] = strings.TrimLeft(line[1:], " ")
		}
	}

	return strings.Join(lines, "\n")
}

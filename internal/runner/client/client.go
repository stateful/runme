package client

import (
	"context"
	"fmt"
	"io"

	runnerv1 "github.com/stateful/runme/internal/gen/proto/go/runme/runner/v1"
	"github.com/stateful/runme/internal/project"
	"github.com/stateful/runme/internal/runner"
	"go.uber.org/zap"
)

type RunnerOption func(Runner) error

var ErrRunnerClientUnimplemented = fmt.Errorf("method unimplemented")

type Runner interface {
	setSession(s *runner.Session) error
	setSessionID(id string) error
	setProject(p project.Project) error
	setCleanupSession(cleanup bool) error
	setSessionStrategy(runnerv1.SessionStrategy) error

	setWithinShell() error
	setDir(dir string) error

	setStdin(stdin io.Reader) error
	setStdout(stdout io.Writer) error
	setStderr(stdout io.Writer) error

	setEnvs(envs []string) error

	getStdin() io.Reader
	getStdout() io.Writer
	getStderr() io.Writer

	setLogger(logger *zap.Logger) error
	setEnableBackgroundProcesses(disableBackground bool) error

	setInsecure(insecure bool) error
	setTLSDir(tlsDir string) error

	RunBlock(ctx context.Context, block project.FileCodeBlock) error
	DryRunBlock(ctx context.Context, block project.FileCodeBlock, w io.Writer, opts ...RunnerOption) error
	Cleanup(ctx context.Context) error

	Clone() Runner
}

func WithSession(s *runner.Session) RunnerOption {
	return func(rc Runner) error {
		return rc.setSession(s)
	}
}

func WithSessionID(id string) RunnerOption {
	return func(rc Runner) error {
		return rc.setSessionID(id)
	}
}

func WithProject(proj project.Project) RunnerOption {
	return func(rc Runner) error {
		return rc.setProject(proj)
	}
}

func WithCleanupSession(cleanup bool) RunnerOption {
	return func(rc Runner) error {
		return rc.setCleanupSession(cleanup)
	}
}

func WithSessionStrategy(strategy runnerv1.SessionStrategy) RunnerOption {
	return func(rc Runner) error {
		return rc.setSessionStrategy(strategy)
	}
}

func WithinShellMaybe() RunnerOption {
	return func(rc Runner) error {
		return rc.setWithinShell()
	}
}

func WithDir(dir string) RunnerOption {
	return func(rc Runner) error {
		return rc.setDir(dir)
	}
}

func WithStdin(stdin io.Reader) RunnerOption {
	return func(rc Runner) error {
		return rc.setStdin(stdin)
	}
}

func WithStdout(stdout io.Writer) RunnerOption {
	return func(rc Runner) error {
		return rc.setStdout(stdout)
	}
}

func WithStderr(stderr io.Writer) RunnerOption {
	return func(rc Runner) error {
		return rc.setStderr(stderr)
	}
}

func WithStdinTransform(op func(io.Reader) io.Reader) RunnerOption {
	return func(rc Runner) error {
		return rc.setStdin(op(rc.getStdin()))
	}
}

func WithStdoutTransform(op func(io.Writer) io.Writer) RunnerOption {
	return func(rc Runner) error {
		return rc.setStdout(op(rc.getStdout()))
	}
}

func WithStderrTransform(op func(io.Writer) io.Writer) RunnerOption {
	return func(rc Runner) error {
		return rc.setStderr(op(rc.getStderr()))
	}
}

func WithLogger(logger *zap.Logger) RunnerOption {
	return func(rc Runner) error {
		return rc.setLogger(logger)
	}
}

func WithInsecure(insecure bool) RunnerOption {
	return func(rc Runner) error {
		return rc.setInsecure(insecure)
	}
}

func WithTLSDir(tlsDir string) RunnerOption {
	return func(rc Runner) error {
		return rc.setTLSDir(tlsDir)
	}
}

func WithEnableBackgroundProcesses(disableBackground bool) RunnerOption {
	return func(rc Runner) error {
		return rc.setEnableBackgroundProcesses(disableBackground)
	}
}

func WithEnvs(envs []string) RunnerOption {
	return func(rc Runner) error {
		return rc.setEnvs(envs)
	}
}

func ApplyOptions(rc Runner, opts ...RunnerOption) error {
	for _, opt := range opts {
		if err := opt(rc); err != nil {
			return err
		}
	}

	return nil
}

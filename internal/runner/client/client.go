package client

import (
	"context"
	"fmt"
	"io"

	"github.com/stateful/runme/internal/document"
	runnerv1 "github.com/stateful/runme/internal/gen/proto/go/runme/runner/v1"
	"github.com/stateful/runme/internal/runner"
	"go.uber.org/zap"
)

type RunnerOption func(Runner) error

var ErrRunnerClientUnimplemented = fmt.Errorf("method unimplemented")

type Runner interface {
	setSession(s *runner.Session) error
	setSessionID(id string) error
	setCleanupSession(cleanup bool) error
	setSessionStrategy(runnerv1.SessionStrategy) error

	setWithinShell() error
	setDir(dir string) error
	setStdin(stdin io.Reader) error
	setStdout(stdout io.Writer) error
	setStderr(stdout io.Writer) error
	setLogger(logger *zap.Logger) error

	SetInsecure(insecure bool) error
	setTLSDir(tlsDir string) error

	RunBlock(ctx context.Context, block *document.CodeBlock) error
	DryRunBlock(ctx context.Context, block *document.CodeBlock, w io.Writer, opts ...RunnerOption) error
	Cleanup(ctx context.Context) error
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

func WithLogger(logger *zap.Logger) RunnerOption {
	return func(rc Runner) error {
		return rc.setLogger(logger)
	}
}

func WithInsecure(insecure bool) RunnerOption {
	return func(rc Runner) error {
		return rc.SetInsecure(insecure)
	}
}

func WithTLSDir(tlsDir string) RunnerOption {
	return func(rc Runner) error {
		return rc.setTLSDir(tlsDir)
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

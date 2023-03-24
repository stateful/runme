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
	runnerv1 "github.com/stateful/runme/internal/gen/proto/go/runme/runner/v1"
	"github.com/stateful/runme/internal/runner"
	"go.uber.org/zap"
)

type LocalRunner struct {
	dir    string
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer

	shellID int
	session *runner.Session

	logger *zap.Logger
}

func (r *LocalRunner) setSession(s *runner.Session) error {
	r.session = s
	return nil
}

func (r *LocalRunner) setSessionID(sessionID string) error {
	return nil
}

func (r *LocalRunner) setCleanupSession(cleanup bool) error {
	return nil
}

func (r *LocalRunner) setSessionStrategy(runnerv1.SessionStrategy) error {
	return nil
}

func (r *LocalRunner) setWithinShell() error {
	id, ok := shellID()
	if !ok {
		return nil
	}
	r.shellID = id
	return nil
}

func (r *LocalRunner) setDir(dir string) error {
	r.dir = dir
	return nil
}

func (r *LocalRunner) setStdin(stdin io.Reader) error {
	r.stdin = stdin
	return nil
}

func (r *LocalRunner) setStdout(stdout io.Writer) error {
	r.stdout = stdout
	return nil
}

func (r *LocalRunner) setStderr(stderr io.Writer) error {
	r.stderr = stderr
	return nil
}

func (r *LocalRunner) setLogger(logger *zap.Logger) error {
	r.logger = logger
	return nil
}

func (r *LocalRunner) SetInsecure(bool) error {
	return nil
}

func (r *LocalRunner) setTLSDir(string) error {
	return nil
}

func NewLocalRunner(opts ...RunnerOption) (*LocalRunner, error) {
	r := &LocalRunner{}
	if err := ApplyOptions(r, opts...); err != nil {
		return nil, err
	}

	if r.logger == nil {
		r.logger = zap.NewNop()
	}

	r.session = runner.NewSession(os.Environ(), r.logger)

	return r, nil
}

func (r *LocalRunner) newExecutable(block *document.CodeBlock) runner.Executable {
	cfg := &runner.ExecutableConfig{
		Name:    block.Name(),
		Dir:     r.dir,
		Tty:     block.Interactive(),
		Stdout:  r.stdout,
		Stderr:  r.stderr,
		Session: r.session,
		Logger:  r.logger,
	}

	if block.Interactive() {
		cfg.Stdin = r.stdin
	}

	switch block.Language() {
	// TODO(mxs): empty string should return nil when guesslang model is implemented
	case "bash", "bat", "sh", "shell", "zsh", "":
		return &runner.Shell{
			ExecutableConfig: cfg,
			Cmds:             block.Lines(),
		}
	case "sh-raw":
		return &runner.ShellRaw{
			Shell: &runner.Shell{
				ExecutableConfig: cfg,
				Cmds:             block.Lines(),
			},
		}
	case "go":
		return &runner.Go{
			ExecutableConfig: cfg,
			Source:           string(block.Content()),
		}
	default:
		return nil
	}
}

func (r *LocalRunner) RunBlock(ctx context.Context, block *document.CodeBlock) error {
	if r.shellID > 0 {
		return r.runBlockInShell(ctx, block)
	}

	executable := r.newExecutable(block)
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

func (r *LocalRunner) DryRunBlock(ctx context.Context, block *document.CodeBlock, w io.Writer, opts ...RunnerOption) error {
	executable := r.newExecutable(block)

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

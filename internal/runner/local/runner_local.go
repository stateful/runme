package local

import (
	"context"
	"io"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/stateful/runme/internal/document"
	"github.com/stateful/runme/internal/runner"
	"go.uber.org/zap"
)

type Option func(*Runner) error

func WithSession(s *runner.Session) Option {
	return func(r *Runner) error {
		r.session = s
		return nil
	}
}

func WithinShellMaybe() Option {
	return func(r *Runner) error {
		id, ok := shellID()
		if !ok {
			return nil
		}
		r.shellID = id
		return nil
	}
}

func WithDir(dir string) Option {
	return func(r *Runner) error {
		r.dir = dir
		return nil
	}
}

func WithStdin(stdin io.Reader) Option {
	return func(r *Runner) error {
		r.stdin = stdin
		return nil
	}
}

func WithStdout(stdout io.Writer) Option {
	return func(r *Runner) error {
		r.stdout = stdout
		return nil
	}
}

func WithStderr(stderr io.Writer) Option {
	return func(r *Runner) error {
		r.stderr = stderr
		return nil
	}
}

type Runner struct {
	dir    string
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer

	shellID int
	session *runner.Session

	logger *zap.Logger
}

func New(opts ...Option) (*Runner, error) {
	r := &Runner{}

	for _, o := range opts {
		if err := o(r); err != nil {
			return nil, err
		}
	}

	if r.logger == nil {
		r.logger = zap.NewNop()
	}

	return r, nil
}

func (r *Runner) newExecutable(block *document.CodeBlock) runner.Executable {
	cfg := &runner.ExecutableConfig{
		Name:    block.Name(),
		Dir:     r.dir,
		Tty:     block.Interactive(),
		Stdin:   r.stdin,
		Stdout:  r.stdout,
		Stderr:  r.stderr,
		Session: r.session,
		Logger:  r.logger,
	}

	switch block.Language() {
	case "bash", "bat", "sh", "shell", "zsh":
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

func (r *Runner) RunBlock(ctx context.Context, block *document.CodeBlock) error {
	if r.shellID > 0 {
		return r.runBlockInShell(ctx, block)
	}

	executable := r.newExecutable(block)
	if executable == nil {
		return errors.Errorf("unknown executable: %q", block.Language())
	}

	return errors.WithStack(executable.Run(ctx))
}

func (r *Runner) runBlockInShell(ctx context.Context, block *document.CodeBlock) error {
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

func (r *Runner) DryRunBlock(ctx context.Context, block *document.CodeBlock, w io.Writer, opts ...Option) error {
	executable := r.newExecutable(block)
	if executable == nil {
		return errors.Errorf("unknown executable: %q", block.Language())
	}

	executable.DryRun(ctx, w)

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

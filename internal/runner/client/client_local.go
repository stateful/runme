package client

import (
	"context"
	"io"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/stateful/runme/v3/internal/document"
	runnerv1 "github.com/stateful/runme/v3/internal/gen/proto/go/runme/runner/v1"
	"github.com/stateful/runme/v3/internal/project"
	"github.com/stateful/runme/v3/internal/runner"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type LocalRunner struct {
	*RunnerSettings

	shellID int

	Stop func()
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

	lis, err := r.setupAdHocServer()
	if err != nil {
		return nil, err
	}

	if err := r.newRunnerServiceClient(lis); err != nil {
		return nil, err
	}

	ctx := context.Background()
	if err := setupSession(r.RunnerSettings, ctx); err != nil {
		return nil, err
	}

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

	cfg := &runner.ExecutableConfig{
		Name:   block.Name(),
		Dir:    r.dir,
		Tty:    block.Interactive(),
		Stdout: r.stdout,
		Stderr: r.stderr,
		Logger: r.logger,
	}

	cfg.PreEnv, err = r.project.LoadEnv()
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
	return runTask(r.RunnerSettings, ctx, task)
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

func (r *LocalRunner) setupAdHocServer() (
	interface{ Dial() (net.Conn, error) },
	error,
) {
	lis := bufconn.Listen(1024 << 10)
	server := grpc.NewServer()

	rss, err := runner.NewRunnerService(r.logger)
	if err != nil {
		return nil, err
	}

	runnerv1.RegisterRunnerServiceServer(server, rss)
	go server.Serve(lis)
	r.Stop = server.Stop

	return lis, nil
}

func (r *LocalRunner) newRunnerServiceClient(
	lis interface{ Dial() (net.Conn, error) },
) error {
	conn, err := grpc.Dial(
		"",
		grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return err
	}

	r.client = runnerv1.NewRunnerServiceClient(conn)
	return nil
}

func (r *LocalRunner) GetEnvs(ctx context.Context) ([]string, error) {
	return getEnvs(r.RunnerSettings, ctx)
}

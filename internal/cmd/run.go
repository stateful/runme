package cmd

import (
	"context"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/pkg/errors"
	"github.com/rwtodd/Go.Sed/sed"
	"github.com/spf13/cobra"
	"github.com/stateful/runme/internal/document"
	"github.com/stateful/runme/internal/runner"
	"go.uber.org/zap"
)

type runCmdOpts struct {
	DryRun         bool
	ReplaceScripts []string
}

func runCmd() *cobra.Command {
	opts := runCmdOpts{}

	cmd := cobra.Command{
		Use:               "run",
		Aliases:           []string{"exec"},
		Short:             "Run a selected command",
		Long:              "Run a selected command identified based on its unique parsed name.",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: validCmdNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			blocks, err := getCodeBlocks()
			if err != nil {
				return err
			}

			block, err := lookupCodeBlock(blocks, args[0])
			if err != nil {
				return err
			}

			return runBlock(cmd, block, nil, &opts)
		},
	}

	setDefaultFlags(&cmd)

	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Print the final command without executing.")
	cmd.Flags().StringArrayVarP(&opts.ReplaceScripts, "replace", "r", nil, "Replace instructions using sed.")

	return &cmd
}

func runBlock(
	cmd *cobra.Command,
	block *document.CodeBlock,
	sess *runner.Session,
	opts *runCmdOpts,
) error {
	if opts == nil {
		opts = &runCmdOpts{}
	}

	if err := replace(opts.ReplaceScripts, block.Lines()); err != nil {
		return err
	}

	if id, ok := shellID(); ok && runner.IsShell(block.Language()) {
		return executeInShell(id, block)
	}

	if sess == nil {
		sess = runner.NewSession(nil, zap.NewNop())
	}

	executable, err := newExecutable(cmd, block, sess)
	if err != nil {
		return err
	}

	ctx, cancel := ctxWithSigCancel(cmd.Context())
	defer cancel()

	if opts.DryRun {
		executable.DryRun(ctx, cmd.ErrOrStderr())
		return nil
	}

	return errors.WithStack(executable.Run(ctx))
}

func newExecutable(cmd *cobra.Command, block *document.CodeBlock, sess *runner.Session) (runner.Executable, error) {
	tty, _ := strconv.ParseBool(block.Attributes()["interactive"])

	cfg := &runner.ExecutableConfig{
		Name:    block.Name(),
		Dir:     fChdir,
		Tty:     tty,
		Stdin:   cmd.InOrStdin(),
		Stdout:  cmd.OutOrStdout(),
		Stderr:  cmd.ErrOrStderr(),
		Session: sess,
		Logger:  zap.NewNop(),
	}

	switch block.Language() {
	case "bash", "bat", "sh", "shell", "zsh":
		return &runner.Shell{
			ExecutableConfig: cfg,
			Cmds:             block.Lines(),
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
		return nil, errors.Errorf("unknown executable: %q", block.Language())
	}
}

func ctxWithSigCancel(ctx context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ctx)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		cancel()
	}()

	return ctx, cancel
}

func replace(scripts []string, lines []string) error {
	if len(scripts) == 0 {
		return nil
	}

	for _, script := range scripts {
		engine, err := sed.New(strings.NewReader(script))
		if err != nil {
			return errors.Wrapf(err, "failed to compile sed script %q", script)
		}

		for idx, line := range lines {
			var err error
			lines[idx], err = engine.RunString(line)
			if err != nil {
				return errors.Wrapf(err, "failed to run sed script %q on line %q", script, line)
			}
		}
	}

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

func executeInShell(id int, block *document.CodeBlock) error {
	conn, err := net.Dial("unix", "/tmp/runme-"+strconv.Itoa(id)+".sock")
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

package cmd

import (
	"context"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/stateful/runme/internal/document"

	"github.com/pkg/errors"
	"github.com/rwtodd/Go.Sed/sed"
	"github.com/spf13/cobra"
	"github.com/stateful/runme/internal/runner"
)

type runCmdOpts struct {
	dryRun         bool
	replaceScripts []string
}

func runCmd() *cobra.Command {
	opts := runCmdOpts{}

	cmd := cobra.Command{
		Use:               "run",
		Aliases:           []string{"exec"},
		Short:             "Run a selected command.",
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

			return runBlock(cmd, block, &opts)
		},
	}

	setDefaultFlags(&cmd)

	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "Print the final command without executing.")
	cmd.Flags().StringArrayVarP(&opts.replaceScripts, "replace", "r", nil, "Replace instructions using sed.")

	return &cmd
}

func runBlock(cmd *cobra.Command, block *document.CodeBlock, opts *runCmdOpts) error {
	if opts == nil {
		opts = &runCmdOpts{}
	}

	if err := replace(opts.replaceScripts, block.Lines()); err != nil {
		return err
	}

	if id, ok := shellID(); ok && runner.IsShell(block) {
		return executeInShell(id, block)
	}

	executable, err := newExecutable(cmd, block)
	if err != nil {
		return err
	}

	ctx, cancel := ctxWithSigCancel(cmd.Context())
	defer cancel()

	if opts.dryRun {
		executable.DryRun(ctx, cmd.ErrOrStderr())
		return nil
	}

	return errors.WithStack(executable.Run(ctx))
}

func newExecutable(cmd *cobra.Command, block *document.CodeBlock) (runner.Executable, error) {
	base := &runner.Base{
		Dir:    fChdir,
		Stdin:  cmd.InOrStdin(),
		Stdout: cmd.OutOrStdout(),
		Stderr: cmd.ErrOrStderr(),
		Name:   block.Name(),
	}

	switch block.Language() {
	case "bash", "bat", "sh", "shell", "zsh":
		return &runner.Shell{
			Cmds: block.Lines(),
			Base: base,
		}, nil
	case "sh-raw":
		return &runner.ShellRaw{
			Cmds: block.Lines(),
			Base: base,
		}, nil
	case "go":
		return &runner.Go{
			Source: string(block.Content()),
			Base:   base,
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

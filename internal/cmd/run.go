package cmd

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/pkg/errors"
	"github.com/rwtodd/Go.Sed/sed"
	"github.com/spf13/cobra"
	"github.com/stateful/rdme/internal/parser"
	"github.com/stateful/rdme/internal/runner"
)

func runCmd() *cobra.Command {
	var (
		dryRun         bool
		replaceScripts []string
	)

	cmd := cobra.Command{
		Use:               "run",
		Aliases:           []string{"exec"},
		Short:             "Run a selected command.",
		Long:              "Run a selected command identified based on its unique parsed name.",
		ValidArgsFunction: validCmdNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := newParser()
			if err != nil {
				return err
			}

			snippet, err := lookup(p.Snippets(), args[0])
			if err != nil {
				return err
			}

			if err := replace(snippet, replaceScripts); err != nil {
				return err
			}

			if err := snippet.FillInParameters(args[1:]); err != nil {
				return err
			}

			executable, err := newExecutable(cmd, snippet)
			if err != nil {
				return err
			}

			ctx, cancel := sigCtxCancel(cmd.Context())
			defer cancel()

			if dryRun {
				executable.DryRun(ctx, cmd.ErrOrStderr())
				return nil
			}

			return errors.WithStack(executable.Run(ctx))
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print the final command without executing.")
	cmd.Flags().StringArrayVarP(&replaceScripts, "replace", "r", nil, "Replace instructions using sed.")

	return &cmd
}

func newExecutable(cmd *cobra.Command, s *parser.Snippet) (runner.Executable, error) {
	switch s.Executable() {
	case "sh":
		return &runner.Shell{
			Cmds: s.Lines(),
			Base: runner.Base{
				Dir:    chdir,
				Stdin:  cmd.InOrStdin(),
				Stdout: cmd.OutOrStdout(),
				Stderr: cmd.ErrOrStderr(),
			},
		}, nil
	case "go":
		return &runner.Go{
			Source: s.Content(),
			Base: runner.Base{
				Dir:    chdir,
				Stdin:  cmd.InOrStdin(),
				Stdout: cmd.OutOrStdout(),
				Stderr: cmd.ErrOrStderr(),
			},
		}, nil
	default:
		return nil, errors.Errorf("unknown executable: %q", s.Executable())
	}
}

func sigCtxCancel(ctx context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ctx)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		cancel()
	}()

	return ctx, cancel
}

func replace(snippet *parser.Snippet, scripts []string) error {
	if len(scripts) == 0 {
		return nil
	}

	content := snippet.Content()

	for _, script := range scripts {
		engine, err := sed.New(strings.NewReader(script))
		if err != nil {
			return errors.Wrapf(err, "failed to compile sed script %q", script)
		}

		content, err = engine.RunString(content)
		if err != nil {
			return errors.Wrapf(err, "failed to run sed script %q", script)
		}
	}

	snippet.ReplaceContent(content)

	return nil
}

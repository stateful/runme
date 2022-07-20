package cmd

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/stateful/rdme/internal/parser"
	"github.com/stateful/rdme/internal/runner"
)

func runCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:     "run",
		Aliases: []string{"exec"},
		Short:   "Run a selected command.",
		Args:    cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			p, err := newParser()
			if err != nil {
				cmd.PrintErrf("failed to get parser: %s", err)
				return nil, cobra.ShellCompDirectiveError
			}

			names := p.Snippets().Names()

			var filtered []string
			for _, name := range names {
				if strings.HasPrefix(name, toComplete) {
					filtered = append(filtered, name)
				}
			}

			return filtered, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := newParser()
			if err != nil {
				return errors.Wrap(err, "fail to read README file")
			}

			snippets := p.Snippets()

			snippet, found := snippets.Lookup(args[0])
			if !found {
				return errors.Errorf("command %q not found; known command names: %s", args[0], snippets.Names())
			}

			ctx, cancel := context.WithCancel(cmd.Context())

			sigs := make(chan os.Signal, 1)
			signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

			go func() {
				<-sigs
				cancel()
			}()

			executable, err := newExecutable(cmd, snippet)
			if err != nil {
				return errors.WithStack(err)
			}

			return errors.WithStack(executable.Run(ctx))
		},
	}

	return &cmd
}

func newExecutable(cmd *cobra.Command, s parser.Snippet) (runner.Executable, error) {
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

package cmd

import (
	"context"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
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

			sh, ok := os.LookupEnv("SHELL")
			if !ok {
				sh = "/bin/sh"
			}

			stdin := cmd.InOrStdin()
			stdout, stderr := cmd.OutOrStdout(), cmd.ErrOrStderr()

			for _, cmd := range snippet.Cmds() {
				if err := execSingle(ctx, sh, cmd, stdin, stdout, stderr); err != nil {
					return err
				}
			}

			return nil
		},
	}

	return &cmd
}

func execSingle(ctx context.Context, sh, cmd string, stdin io.Reader, stdout, stderr io.Writer) error {
	c := exec.CommandContext(ctx, sh, []string{"-c", cmd}...)
	c.Dir = chdir
	c.Stderr = stderr
	c.Stdout = stdout
	c.Stdin = stdin

	return errors.Wrapf(c.Run(), "failed to run command %q", cmd)
}

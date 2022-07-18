package cmd

import (
	"context"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/google/shlex"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/stateful/rdme/internal/parser"
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
			// TODO: like can omit because Args should do the validation.
			if len(args) != 1 {
				return cmd.Help()
			}

			p, err := newParser()
			if err != nil {
				return errors.Wrap(err, "fail to read README file")
			}

			snippets := p.Snippets()

			snippet, found := p.Snippets().Lookup(args[0])
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

			if len(snippet.Cmds()) == 1 {
				return execSingle(ctx, sh, snippet.FirstCmd(), stdin, stdout, stderr)
			}

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
	fragments, err := shlex.Split(cmd)
	if err != nil {
		return errors.Wrapf(err, "failed to parse command %q", cmd)
	}

	c := exec.CommandContext(ctx, sh, []string{"-c", strings.Join(fragments, " ")}...)
	c.Dir = chdir
	c.Stderr = stderr
	c.Stdout = stdout
	c.Stdin = stdin

	return errors.Wrapf(c.Run(), "failed to run command %s", cmd)
}

func newParser() (*parser.Parser, error) {
	source, err := os.ReadFile(filepath.Join(chdir, fileName))
	if err != nil {
		return nil, errors.Wrap(err, "fail to read README file")
	}

	return parser.New(source), nil
}

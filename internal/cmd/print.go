package cmd

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func printCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:               "print",
		Short:             "Print a selected snippet.",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: validCmdNames,
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

			_, err = cmd.OutOrStdout().Write([]byte(snippet.Content()))
			return errors.Wrap(err, "failed to write to stdout")
		},
	}

	return &cmd
}

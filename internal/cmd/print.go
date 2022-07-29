package cmd

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func printCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:               "print",
		Short:             "Print a selected snippet.",
		Long:              "Print will display the details of the corresponding command block based on its name.",
		Args:              cobra.ExactArgs(1),
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

			_, err = cmd.OutOrStdout().Write([]byte(snippet.Content()))
			return errors.Wrap(err, "failed to write to stdout")
		},
	}

	return &cmd
}

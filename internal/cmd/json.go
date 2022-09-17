package cmd

import (
	"github.com/spf13/cobra"
)

func jsonCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:    "json",
		Short:  "Generates json. Caution, this is experimental.",
		Hidden: true,
		Args:   cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			return renderToJSON(cmd.OutOrStdout())
		},
	}
	return &cmd
}

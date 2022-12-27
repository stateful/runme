package cmd

import (
	"github.com/spf13/cobra"
)

func daemonCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:   "daemon",
		Short: "Start a daemon with a shell session.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	setDefaultFlags(&cmd)

	return &cmd
}

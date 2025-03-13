package server

import (
	"github.com/spf13/cobra"

	"github.com/runmedev/runme/v3/internal/config"
	"github.com/runmedev/runme/v3/internal/config/autoconfig"
)

func Cmd() *cobra.Command {
	cmd := cobra.Command{
		Use:    "server",
		Short:  "Commands to manage and call a runme server.",
		Hidden: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return autoconfig.Invoke(
				func(
					cfg *config.Config,
				) error {
					// For the server commands, we want to always log to stdout.
					// TODO(adamb): there might be a need to separate client and server logs.
					cfg.Log.Path = ""
					return nil
				},
			)
		},
	}

	cmd.AddCommand(serverGRPCurlCmd())
	cmd.AddCommand(serverStartCmd())
	cmd.AddCommand(serverStopCmd())

	return &cmd
}

package beta

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func BetaCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:   "beta",
		Short: "Experimental commands.",
		Long: `Experimental commands that are not yet ready for production use.

All commands use the runme.yaml configuration file.`,
		Hidden: true,
	}

	// Hide all persistent flags from the root command.
	// "beta" is a completely different set of commands and
	// should not inherit any flags from the root command.
	originalUsageFunc := cmd.UsageFunc()
	cmd.SetUsageFunc(func(cmd *cobra.Command) error {
		pflags := cmd.Root().PersistentFlags()

		pflags.VisitAll(func(f *pflag.Flag) {
			f.Hidden = true
		})

		return originalUsageFunc(cmd)
	})

	cmd.AddCommand(runLocallyCmd())

	return &cmd
}

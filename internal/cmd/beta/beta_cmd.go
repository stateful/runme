package beta

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/stateful/runme/v3/internal/cmd/beta/server"
	"github.com/stateful/runme/v3/internal/config"
	"github.com/stateful/runme/v3/internal/config/autoconfig"
)

type commonFlags struct {
	categories []string
	filename   string
}

func BetaCmd() *cobra.Command {
	cFlags := &commonFlags{}

	cmd := cobra.Command{
		Use:   "beta",
		Short: "Experimental runme commands.",
		Long: `The new version of the runme command-line interface.

All commands are experimental and not yet ready for production use.

All commands use the runme.yaml configuration file.`,
		Hidden: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return autoconfig.Invoke(func(cfg *config.Config) error {
				// Override the filename if provided.
				if cFlags.filename != "" {
					cfg.Filename = cFlags.filename
				}

				// Add a filter to run only tasks from the specified categories.
				if len(cFlags.categories) > 0 {
					cfg.Filters = append(cfg.Filters, &config.Filter{
						Type:      config.FilterTypeBlock,
						Condition: `len(intersection(categories, extra.categories)) > 0`,
						Extra:     map[string]interface{}{"categories": cFlags.categories},
					})
				}

				return nil
			})
		},
	}

	pFlags := cmd.PersistentFlags()
	pFlags.StringVar(&cFlags.filename, "filename", "", "Name of the Markdown file to run blocks from.")
	pFlags.StringSliceVar(&cFlags.categories, "category", nil, "Run blocks only from listed categories.")

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

	cmd.AddCommand(listCmd(cFlags))
	cmd.AddCommand(printCmd(cFlags))
	cmd.AddCommand(server.Cmd())
	cmd.AddCommand(runCmd(cFlags))

	return &cmd
}

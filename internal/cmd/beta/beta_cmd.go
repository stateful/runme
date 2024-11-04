package beta

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.uber.org/zap"

	"github.com/stateful/runme/v3/internal/cmd/beta/server"
	"github.com/stateful/runme/v3/internal/config"
	"github.com/stateful/runme/v3/internal/config/autoconfig"
)

type commonFlags struct {
	tags     []string
	filename string
	insecure bool
	silent   bool
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
			if cFlags.silent {
				cmd.SetErr(io.Discard)
			}

			err := autoconfig.InvokeForCommand(func(cfg *config.Config, log *zap.Logger) error {
				// Override the filename if provided.
				if cFlags.filename != "" {
					cfg.Project.Filename = cFlags.filename
				}

				// Add a filter to run only tasks from the specified tags.
				if len(cFlags.tags) > 0 {
					cfg.Project.Filters = append(
						cfg.Project.Filters,
						config.ConfigProjectFiltersElem{
							Type:      config.FilterTypeBlock,
							Condition: `len(intersection(tags, extra.tags)) > 0`,
							Extra:     map[string]interface{}{"tags": cFlags.tags},
						},
					)
				}

				log.Info("final configuration", zap.Any("config", cfg))

				return nil
			})

			// Print the error to stderr but don't return it because error modes
			// are neither fully baked yet nor ready for users to consume.
			if err != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%s\n", err)
			}

			return nil
		},
	}

	// The idea for persistent flags on the "beta" command is to
	// interpret them in PersistentPreRunE() and merge with [config.Config].
	// Use them sparingly and only for the cases when it does not make sense
	// to alter the configuration file.
	pFlags := cmd.PersistentFlags()
	pFlags.StringSliceVar(&cFlags.tags, "tag", nil, "Run blocks only from listed tags.")
	pFlags.StringVar(&cFlags.filename, "filename", "", "Name of the Markdown file to run blocks from.")
	pFlags.BoolVar(&cFlags.insecure, "insecure", false, "Explicitly allow delicate operations to prevent misuse")
	pFlags.BoolVar(&cFlags.silent, "silent", false, "Silent mode. Do not print error messages.")

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
	cmd.AddCommand(envCmd(cFlags))

	return &cmd
}

package beta

import (
	"fmt"
	"strings"

	"github.com/gobwas/glob"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/stateful/runme/internal/command"
	"github.com/stateful/runme/internal/config"
	"github.com/stateful/runme/internal/config/autoconfig"
	"github.com/stateful/runme/internal/document"
	"github.com/stateful/runme/internal/project"
)

func runLocallyCmd() *cobra.Command {
	var (
		categories []string
		filename   string
	)

	cmd := cobra.Command{
		Use:   "run [command1 command2 ...]",
		Short: "Run one or more commands.",
		Long: `Run commands by providing their names delimited by space.
The names are interpreted as glob patterns.

In the case of multiple commands, they are executed in the order they appear in the document.

The --category option additionally filters the list of tasks to execute by category.`,
		Example: `Run all blocks starting with the "generate-" prefix:
  runme beta run "generate-*"

Run all blocks from the "setup" and "teardown" categories:
  runme beta run --category=setup,teardown
`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// TODO(adamb): this is an example of how to convert a flag to a ConfigFilter.
			// The custom per-command flags should be reduced to minimum and runme.yaml should
			// be the source of all configuration. In practice, some flags will still be needed
			// for the convenience. Move the implementation to appropriate place.
			return autoconfig.Invoke(func(cfg *config.Config) error {
				if filename != "" {
					cfg.Filename = filename
				}

				if len(categories) > 0 {
					var cBuilder strings.Builder

					cLen := len(categories)
					for i, c := range categories {
						_, _ = cBuilder.WriteString("'")
						_, _ = cBuilder.WriteString(c)
						_, _ = cBuilder.WriteString("'")

						if i < cLen-1 {
							_, _ = cBuilder.WriteString(",")
						}
					}

					cfg.Filters = append(cfg.Filters, &config.Filter{
						Type: config.FilterTypeBlock,
						Condition: fmt.Sprintf(
							// The predicate in `filter` can be read as follows:
							//   * Assign the current processed block's category to `c`.
							//   * Check if any provided category matches `c` using an inner predicate for `any`.
							"len(filter(categories, { let c = #; any([%s], # == c) })) > 0",
							cBuilder.String(),
						),
					})
				}

				return nil
			})
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return autoconfig.Invoke(
				func(
					proj *project.Project,
					filters []project.Filter,
					logger *zap.Logger,
					session *command.Session,
				) error {
					defer logger.Sync()

					tasks, err := project.LoadTasks(cmd.Context(), proj)
					if err != nil {
						return err
					}
					logger.Info("found tasks", zap.Int("count", len(tasks)))

					tasks, err = project.FilterTasksByFn(tasks, filters...)
					if err != nil {
						return err
					}
					logger.Info("filtered tasks by filters", zap.Int("count", len(tasks)))

					tasks, err = filterTasksByGlobs(tasks, args)
					if err != nil {
						return err
					}
					logger.Info("filtered tasks by args", zap.Strings("args", args), zap.Int("count", len(tasks)))

					for _, t := range tasks {
						err := runCommandNatively(cmd, t.CodeBlock, session, logger)
						if err != nil {
							return err
						}
					}

					return nil
				},
			)
		},
	}

	cmd.Flags().StringVar(&filename, "filename", "", "Name of the Markdown file to run blocks from.")
	cmd.Flags().StringSliceVar(&categories, "category", nil, "Run blocks only from listed categories.")

	return &cmd
}

func runCommandNatively(cmd *cobra.Command, block *document.CodeBlock, sess *command.Session, logger *zap.Logger) error {
	cfg, err := command.NewConfigFromCodeBlock(block)
	if err != nil {
		return err
	}

	opts := &command.NativeCommandOptions{
		Session: sess,
		Stdin:   cmd.InOrStdin(),
		Stdout:  cmd.OutOrStdout(),
		Stderr:  cmd.ErrOrStderr(),
		Logger:  logger,
	}

	nativeCommand, err := command.NewNative(cfg, opts)
	if err != nil {
		return err
	}

	err = nativeCommand.Start(cmd.Context())
	if err != nil {
		return err
	}

	return nativeCommand.Wait()
}

func parseGlobs(items []string) ([]glob.Glob, error) {
	globs := make([]glob.Glob, 0, len(items))
	for _, item := range items {
		g, err := glob.Compile(item)
		if err != nil {
			return nil, err
		}
		globs = append(globs, g)
	}
	return globs, nil
}

func filterTasksByGlobs(tasks []project.Task, patterns []string) (result []project.Task, _ error) {
	if len(patterns) == 0 {
		return tasks, nil
	}

	globs, err := parseGlobs(patterns)
	if err != nil {
		return nil, err
	}

	for _, g := range globs {
		match := false

		for _, t := range tasks {
			if g.Match(t.CodeBlock.Name()) {
				result = append(result, t)
				match = true
			}
		}

		if !match {
			return nil, errors.Errorf("no task found for glob %q", g)
		}
	}

	return
}

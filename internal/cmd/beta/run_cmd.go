package beta

import (
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/stateful/runme/v3/internal/command"
	"github.com/stateful/runme/v3/internal/config/autoconfig"
	"github.com/stateful/runme/v3/internal/document"
	"github.com/stateful/runme/v3/internal/project"
)

func runCmd(_ *commonFlags) *cobra.Command {
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

					argsFilter, err := createProjectFilterFromPatterns(args)
					if err != nil {
						return err
					}

					filters = append(filters, argsFilter)

					tasks, err = project.FilterTasksByFn(tasks, filters...)
					if err != nil {
						return err
					}
					logger.Info("filtered tasks by filters", zap.Int("count", len(tasks)))

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

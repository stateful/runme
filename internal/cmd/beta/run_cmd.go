package beta

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/stateful/runme/v3/internal/command"
	"github.com/stateful/runme/v3/internal/config/autoconfig"
	"github.com/stateful/runme/v3/internal/project"
)

func runCmd(*commonFlags) *cobra.Command {
	var kernelName string

	cmd := cobra.Command{
		Use:     "run [command1 command2 ...]",
		Aliases: []string{"exec"},
		Short:   "Run one or more commands.",
		Long: `Run commands by providing their names delimited by space.
The names are interpreted as glob patterns.

In the case of multiple commands, they are executed one-by-one in the order they appear in the document.

The --category option additionally filters the list of tasks to execute by category.`,
		Example: `Run all blocks starting with the "generate-" prefix:
  runme beta run "generate-*"

Run all blocks from the "setup" and "teardown" categories:
  runme beta run --category=setup,teardown
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return autoconfig.Invoke(
				func(
					filters []project.Filter,
					kernelGetter autoconfig.KernelGetter,
					logger *zap.Logger,
					proj *project.Project,
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

					if len(tasks) == 0 {
						_, err := cmd.ErrOrStderr().Write([]byte("no tasks to run\n"))
						return errors.WithStack(err)
					}

					kernel, err := kernelGetter(kernelName)
					if err != nil {
						return err
					}

					for _, t := range tasks {
						err := runCodeBlock(t, cmd, kernel, session, logger)
						if err != nil {
							return err
						}
					}

					return nil
				},
			)
		},
	}

	cmd.Flags().StringVar(&kernelName, "kernel", "", "Kernel name or index to use for command execution.")

	return &cmd
}

func runCodeBlock(
	task project.Task,
	cmd *cobra.Command,
	kernel command.Kernel,
	sess *command.Session,
	logger *zap.Logger,
) error {
	// TODO(adamb): [command.Config] is generated exclusively from the [document.CodeBlock].
	// As we introduce some document- and block-related configs in runme.yaml (root but also nested),
	// this [Command.Config] should be further extended.
	//
	// The way to do it is to use [config.Loader] and calling [config.Loader.FindConfigChain] with
	// task's document path. It will produce all the configs that are relevant to the document.
	// Next, they should be merged into a single [config.Config] in a correct order, starting from
	// the last element of the returned config chain. Finally, [command.Config] should be updated.
	// This algorithm should be likely encapsulated in the [internal/config] and [internal/command]
	// packages.
	cfg, err := command.NewConfigFromCodeBlock(task.CodeBlock)
	if err != nil {
		return err
	}

	kernelCmd := kernel.Command(
		cfg,
		command.Options{
			Kernel:  kernel,
			Logger:  logger,
			Session: sess,
			Stdin:   cmd.InOrStdin(),
			Stdout:  cmd.OutOrStdout(),
			Stderr:  cmd.ErrOrStderr(),
		},
	)

	err = kernelCmd.Start(cmd.Context())
	if err != nil {
		return err
	}

	return kernelCmd.Wait()
}

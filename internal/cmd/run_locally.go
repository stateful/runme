package cmd

import (
	"github.com/gobwas/glob"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/stateful/runme/internal/command"
	"github.com/stateful/runme/internal/document"
	"github.com/stateful/runme/internal/project"
)

func runLocally() *cobra.Command {
	var category string

	cmd := cobra.Command{
		Use:   "run-locally [command1 command2 ...]",
		Short: "Run one or more commands.",
		Long: `Run commands by providing their names delimited by space.
The names are interpreted as glob patterns.

The --category option additionally filters the list of tasks to execute.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && category == "" {
				return errors.New("no commands and no --category provided")
			}

			logger, err := getLogger(true)
			if err != nil {
				return err
			}
			defer logger.Sync()

			session := command.NewSession()

			proj, err := getProject()
			if err != nil {
				return err
			}

			tasks, err := project.LoadTasks(cmd.Context(), proj)
			if err != nil {
				return err
			}

			logger.Debug("found tasks", zap.Int("count", len(tasks)))

			if category != "" {
				tasks = filterTasksByCategory(tasks, category)
				logger.Debug("filtered tasks by category", zap.String("category", category), zap.Int("count", len(tasks)))
			}

			if len(args) > 0 {
				globs := make([]glob.Glob, 0, len(args))
				for _, arg := range args {
					g, err := glob.Compile(arg)
					if err != nil {
						return err
					}
					globs = append(globs, g)
				}

				tasks, err = filterTasksByGlobs(tasks, globs)
				if err != nil {
					return err
				}
				logger.Debug("filtered tasks by globs", zap.Strings("names", args), zap.Int("count", len(tasks)))
			}

			for _, t := range tasks {
				err := runCommandNatively(cmd, t.CodeBlock, session, logger)
				if err != nil {
					return err
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&category, "category", "c", "", "Run from a specific category.")

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

	nativeCmd, err := command.NewNative(cfg, opts)
	if err != nil {
		return err
	}

	err = nativeCmd.Start(cmd.Context())
	if err != nil {
		return err
	}

	return nativeCmd.Wait()
}

func filterTasksByCategory(tasks []project.Task, category string) (result []project.Task) {
	for _, t := range tasks {
		if t.CodeBlock.Category() == category {
			result = append(result, t)
		}
	}
	return
}

func filterTasksByGlobs(tasks []project.Task, globs []glob.Glob) (result []project.Task, _ error) {
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

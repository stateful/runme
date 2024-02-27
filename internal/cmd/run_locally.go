package cmd

import (
	"github.com/gobwas/glob"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/stateful/runme/v3/internal/command"
	"github.com/stateful/runme/v3/internal/document"
	"github.com/stateful/runme/v3/internal/project"
)

func runLocally() *cobra.Command {
	var category string

	cmd := cobra.Command{
		Use:    "run-locally [command1 command2 ...]",
		Hidden: true,
		Short:  "Run one or more commands.",
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

			proj, err := getProject()
			if err != nil {
				return err
			}

			projEnv, err := proj.LoadEnv()
			if err != nil {
				return err
			}

			session, err := command.NewSessionWithEnv(projEnv...)
			if err != nil {
				return err
			}

			tasks, err := project.LoadTasks(cmd.Context(), proj)
			if err != nil {
				return err
			}
			logger.Info("found tasks", zap.Int("count", len(tasks)))

			tasks = filterTasksByCategory(tasks, category)
			logger.Info("filtered tasks by category", zap.String("category", category), zap.Int("count", len(tasks)))

			tasks, err = filterTasksByGlobs(tasks, args)
			if err != nil {
				return err
			}
			logger.Info("filtered tasks by globs", zap.Strings("globs", args), zap.Int("count", len(tasks)))

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

// todo: this is not right; reconcile with the way --category works in run
func filterTasksByCategory(tasks []project.Task, category string) (result []project.Task) {
	if category == "" {
		return tasks
	}
	for _, t := range tasks {
		for _, c := range t.CodeBlock.Categories() {
			if c == category {
				result = append(result, t)
				break
			}
		}
	}
	return
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

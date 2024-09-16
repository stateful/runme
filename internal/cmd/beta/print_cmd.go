package beta

import (
	"io"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/stateful/runme/v3/internal/config/autoconfig"
	"github.com/stateful/runme/v3/pkg/project"
)

func printCmd(*commonFlags) *cobra.Command {
	cmd := cobra.Command{
		Use:   "print [command1 command2 ...]",
		Short: "Print content of commands.",
		Long: `Print commands content by optionally providing their names delimited by space.
The names are interpreted as glob patterns.`,
		Example: `Print content of commands starting with the "generate-" prefix:
  runme beta print "generate-*"

Print content of commands from the "setup" and "teardown" tags:
  runme beta print --tag=setup,teardown
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return autoconfig.InvokeForCommand(
				func(
					proj *project.Project,
					filters []project.Filter,
					logger *zap.Logger,
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

					return printTasksContent(cmd.OutOrStdout(), tasks)
				},
			)
		},
	}

	return &cmd
}

func printTasksContent(w io.Writer, tasks []project.Task) error {
	for _, t := range tasks {
		_, err := w.Write([]byte("# " + project.GetRelativePath(getCwd(), t.DocumentPath) + ":" + t.CodeBlock.Name() + "\n"))
		if err != nil {
			return errors.WithStack(err)
		}
		// TODO(adamb): this is not well-formatted output. It should go through some normalizer/formatter.
		// This does not look great especially for snippets that start with "$".
		_, err = w.Write([]byte(strings.Join(t.CodeBlock.Lines(), "\n")))
		if err != nil {
			return errors.WithStack(err)
		}
		_, err = w.Write([]byte{'\n'})
		if err != nil {
			return err
		}
	}
	return nil
}

package cmd

import (
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/stateful/rdme/internal/tasks"
)

func tasksCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:    "tasks",
		Short:  "Generates task.json for VS Code editor. Caution, this is experimental.",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := newParser()
			if err != nil {
				return err
			}

			snippet, err := lookup(p.Snippets(), args[0])
			if err != nil {
				return err
			}

			tasksDef, err := tasks.GenerateFromShellCommand(
				snippet.Name(),
				snippet.FirstLine(),
				&tasks.ShellCommandOpts{
					Cwd: chdir,
				},
			)
			if err != nil {
				return errors.Wrap(err, "failed to generate tasks.json")
			}

			encoder := json.NewEncoder(cmd.OutOrStdout())
			encoder.SetIndent("", "  ")
			if err := encoder.Encode(tasksDef); err != nil {
				return errors.Wrap(err, "failed to marshal tasks.json")
			}

			return nil
		},
	}
	return &cmd
}

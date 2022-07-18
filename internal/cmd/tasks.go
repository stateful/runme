package cmd

import (
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/stateful/rdme/internal/tasks"
)

func tasksCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:   "tasks",
		Short: "Generates task.json for VS Code editor. Caution, this is experimental.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: like can omit because Args should do the validation.
			if len(args) != 1 {
				return cmd.Help()
			}

			p, err := newParser()
			if err != nil {
				return errors.Wrap(err, "fail to read README file")
			}

			snippets := p.Snippets()

			snippet, found := p.Snippets().Lookup(args[0])
			if !found {
				return errors.Errorf("command %q not found; known command names: %s", args[0], snippets.Names())
			}

			tasksDef, err := tasks.GenerateFromShellCommand(
				snippet.Name(),
				snippet.FirstCmd(),
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

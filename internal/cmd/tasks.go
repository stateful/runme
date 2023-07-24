package cmd

import (
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/stateful/runme/internal/project"
	"github.com/stateful/runme/internal/tasks"
)

func tasksCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:    "tasks",
		Short:  "Generates task.json for VS Code editor. Caution, this is experimental.",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, err := getProject()
			if err != nil {
				return err
			}

		generateBlocks:
			blocks, err := loadTasks(proj, cmd.OutOrStdout(), cmd.InOrStdin(), true)
			if err != nil {
				return err
			}

			block, err := lookupCodeBlockWithPrompt(cmd, args[0], blocks)
			if err != nil {
				if project.IsCodeBlockNotFoundError(err) && !fAllowUnnamed {
					fAllowUnnamed = true
					goto generateBlocks
				}

				return err
			}

			tasksDef, err := tasks.GenerateFromShellCommand(
				block.Block.Name(),
				block.Block.Lines()[0],
				&tasks.ShellCommandOpts{
					Cwd: fChdir,
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

	setDefaultFlags(&cmd)

	return &cmd
}

package cmd

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/cli/cli/v2/utils"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/stateful/runme/internal/project"
	"github.com/stateful/runme/internal/shell"
)

func listCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:     "list [search]",
		Aliases: []string{"ls"},
		Short:   "List available commands",
		Long:    "Displays list of parsed command blocks, their name, number of commands in a block, and description from a given markdown file, such as README.md. Provide an argument to filter results by file and name using a regular expression.",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var regex *regexp.Regexp
			if len(args) > 0 {
				reg, err := regexp.Compile(args[0])
				if err != nil {
					return errors.Wrapf(err, "invalid regex %q", args[0])
				}

				regex = reg
			}

			proj, err := getProject()
			if err != nil {
				return err
			}

			allBlocks, err := proj.LoadTasks()
			if err != nil {
				return err
			}

			var blocks []project.CodeBlock

			if regex == nil {
				blocks = allBlocks
			} else {
				blocks = make([]project.CodeBlock, 0)
				for _, block := range allBlocks {
					id := fmt.Sprintf("%s:%s", block.File, block.Block.Name())

					if !regex.MatchString(id) {
						continue
					}

					blocks = append(blocks, block)
				}
			}

			// TODO: this should be taken from cmd.
			io := iostreams.System()
			//lint:ignore SA1019 utils is deprecated but that's ok for now.
			table := utils.NewTablePrinter(io)

			// table header
			table.AddField(strings.ToUpper("Name"), nil, nil)
			table.AddField(strings.ToUpper("File"), nil, nil)
			table.AddField(strings.ToUpper("First Command"), nil, nil)
			table.AddField(strings.ToUpper("# of Commands"), nil, nil)
			table.AddField(strings.ToUpper("Description"), nil, nil)
			table.EndRow()

			for _, fileBlock := range blocks {
				block := fileBlock.Block

				lines := block.Lines()

				table.AddField(block.Name(), nil, nil)
				table.AddField(fileBlock.File, nil, nil)
				table.AddField(shell.TryGetNonCommentLine(lines), nil, nil)
				table.AddField(fmt.Sprintf("%d", len(shell.StripComments(lines))), nil, nil)
				table.AddField(block.Intro(), nil, nil)
				table.EndRow()
			}

			return errors.Wrap(table.Render(), "failed to render")
		},
	}

	setDefaultFlags(&cmd)

	return &cmd
}

package cmd

import (
	"fmt"
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
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List available commands",
		Long:    "Displays list of parsed command blocks, their name, number of commands in a block, and description from a given markdown file, such as README.md.",
		RunE: func(cmd *cobra.Command, args []string) error {
			p := project.New(fChdir)
			blocks := project.CodeBlocks{}
			var blockError error
			if isInExperimentalMode() {
				blocks, blockError = p.GetAllCodeBlocks(fAllowUnknown, fIgnoreNameless)
			} else {
				codeBlocks, err := p.GetCodeBlocks(fFileName, fAllowUnknown, fIgnoreNameless)
				blocks = append(blocks, &project.FileCodeBlocks{
					FileName:   fFileName,
					CodeBlocks: codeBlocks,
				})
				blockError = err
			}

			if blockError != nil {
				return blockError
			}

			for _, block := range blocks {
				if len(block.CodeBlocks) == 0 {
					continue
				}

				if isInExperimentalMode() {
					_, err := fmt.Println(">>> Commands for", block.FileName)
					if err != nil {
						return err
					}
				}

				// TODO: this should be taken from cmd.
				io := iostreams.System()
				//lint:ignore SA1019 utils is deprecated but that's ok for now.
				table := utils.NewTablePrinter(io)

				// table header
				table.AddField(strings.ToUpper("Name"), nil, nil)
				table.AddField(strings.ToUpper("First Command"), nil, nil)
				table.AddField(strings.ToUpper("# of Commands"), nil, nil)
				table.AddField(strings.ToUpper("Description"), nil, nil)
				table.EndRow()
				for _, code := range block.CodeBlocks {
					lines := code.Lines()

					table.AddField(code.Name(), nil, nil)
					table.AddField(shell.TryGetNonCommentLine(lines), nil, nil)
					table.AddField(fmt.Sprintf("%d", len(shell.StripComments(lines))), nil, nil)
					table.AddField(code.Intro(), nil, nil)
					table.EndRow()
				}

				err := table.Render()
				if err != nil {
					return errors.Wrap(err, "failed to render")
				}

				if isInExperimentalMode() {
					_, err = fmt.Print("\n")
					if err != nil {
						return err
					}
				}
			}

			return nil
		},
	}

	setDefaultFlags(&cmd)

	return &cmd
}

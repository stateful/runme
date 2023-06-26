package cmd

import (
	"path/filepath"
	"sort"
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
			search := ""
			if len(args) > 0 {
				search = args[0]
			}

			proj, err := getProject()
			if err != nil {
				return err
			}

			allBlocks, err := loadTasks(proj, cmd.OutOrStdout(), cmd.InOrStdin())
			if err != nil {
				return err
			}

			blocks, err := allBlocks.LookupByID(search)
			if err != nil {
				return err
			}

			blocks = sortBlocks(blocks)

			// TODO: this should be taken from cmd.
			io := iostreams.System()
			//lint:ignore SA1019 utils is deprecated but that's ok for now.
			table := utils.NewTablePrinter(io)

			// table header
			table.AddField(strings.ToUpper("Name"), nil, nil)
			table.AddField(strings.ToUpper("File"), nil, nil)
			table.AddField(strings.ToUpper("First Command"), nil, nil)
			table.AddField(strings.ToUpper("Description"), nil, nil)
			table.EndRow()

			for _, fileBlock := range blocks {
				block := fileBlock.Block

				lines := block.Lines()

				table.AddField(block.Name(), nil, nil)
				table.AddField(fileBlock.File, nil, nil)
				table.AddField(shell.TryGetNonCommentLine(lines), nil, nil)
				table.AddField(block.Intro(), nil, nil)
				table.EndRow()
			}

			return errors.Wrap(table.Render(), "failed to render")
		},
	}

	setDefaultFlags(&cmd)

	return &cmd
}

// sort blocks in ascending nested order
func sortBlocks(blocks []project.CodeBlock) (res []project.CodeBlock) {
	blocksByFile := make(map[string][]project.CodeBlock, 0)

	files := make([]string, 0)
	for _, fileBlock := range blocks {
		if arr, ok := blocksByFile[fileBlock.File]; ok {
			blocksByFile[fileBlock.File] = append(arr, fileBlock)
			continue
		}

		blocksByFile[fileBlock.File] = []project.CodeBlock{fileBlock}
		files = append(files, fileBlock.File)
	}

	sort.SliceStable(files, func(i, j int) bool {
		return getFileDepth(files[i]) < getFileDepth(files[j])
	})

	for _, file := range files {
		res = append(res, blocksByFile[file]...)
	}

	return
}

func getFileDepth(fp string) int {
	return len(strings.Split(fp, string(filepath.Separator)))
}

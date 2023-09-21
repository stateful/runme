package cmd

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/cli/go-gh/pkg/jsonpretty"
	"github.com/cli/go-gh/pkg/tableprinter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/stateful/runme/internal/project"
	"github.com/stateful/runme/internal/shell"
)

var isJSON bool

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

			allBlocks, err := loadTasks(proj, cmd.OutOrStdout(), cmd.InOrStdin(), true)
			if err != nil {
				return err
			}

			blocks, err := allBlocks.LookupByID(search)
			if err != nil {
				return err
			}

			if len(blocks) <= 0 && !fAllowUnnamed {
				return errors.Errorf("no named code blocks, consider adding flag --allow-unnamed")
			}

			blocks = sortBlocks(blocks)

			// TODO: this should be taken from cmd.
			io := iostreams.System()
            if !isJSON {
                table := tableprinter.New(io.Out, io.IsStdoutTTY(), io.TerminalWidth())

                // table header
                table.AddField(strings.ToUpper("Name"))
                table.AddField(strings.ToUpper("File"))
                table.AddField(strings.ToUpper("First Command"))
                table.AddField(strings.ToUpper("Description"))
                table.AddField(strings.ToUpper("Named"))
                table.EndRow()

                for _, fileBlock := range blocks {
                    block := fileBlock.Block

                    lines := block.Lines()

                    isNamedField := "Yes"
                    if block.IsUnnamed() {
                        isNamedField = "No"
                    }

                    table.AddField(block.Name())
                    table.AddField(fileBlock.File)
                    table.AddField(shell.TryGetNonCommentLine(lines))
                    table.AddField(block.Intro())
                    table.AddField(isNamedField)
                    table.EndRow()
                }

                return errors.Wrap(table.Render(), "failed to render")
            }

            type row struct {
                Name string `json:"name"`
                File string `json:"file"`
                FirstCommand string `json:"first_command"`
                Description string `json:"description"`
                Named bool `json:"named"`
            }
            var rows []row
            for _, fileBlock := range blocks {
                block := fileBlock.Block
                lines := block.Lines()
                r := row{
                    Name: block.Name(),
                    File: fileBlock.File,
                    FirstCommand: shell.TryGetNonCommentLine(lines),
                    Description: block.Intro(),
                    Named: !block.IsUnnamed(),
                }
                rows = append(rows, r)
            }
            by, err := json.Marshal(&rows)
            if err != nil {
                return err
            }
            err = jsonpretty.Format(io.Out, bytes.NewReader(by), "  ", false)
            if err != nil {
                return err
            }
            return nil
		},
	}

    cmd.PersistentFlags().BoolVar(&isJSON, "json", false, "This flag tells the list command to print the output in json")
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

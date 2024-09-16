package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/cli/go-gh/pkg/jsonpretty"
	"github.com/cli/go-gh/pkg/tableprinter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/stateful/runme/v3/internal/shell"
	"github.com/stateful/runme/v3/pkg/project"
)

type row struct {
	Name         string `json:"name"`
	File         string `json:"file"`
	FirstCommand string `json:"first_command"`
	Description  string `json:"description"`
	Named        bool   `json:"named"`
	RunAll       bool   `json:"run_all"`
}

func listCmd() *cobra.Command {
	var formatJSON bool
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

			tasks, err := getProjectTasks(cmd)
			if err != nil {
				return err
			}

			tasks, err = project.FilterTasksByID(tasks, search)
			if err != nil {
				return err
			}

			if len(tasks) <= 0 && !fAllowUnnamed {
				return errors.Errorf("no named code blocks, consider adding flag --allow-unnamed")
			}

			// TODO: this should be taken from cmd.
			io := iostreams.System()
			var rows []row
			for _, task := range tasks {
				block := task.CodeBlock
				lines := block.Lines()
				relPath := project.GetRelativePath(getCwd(), task.DocumentPath)
				r := row{
					Name:         block.Name(),
					File:         relPath,
					FirstCommand: shell.TryGetNonCommentLine(lines),
					Description:  block.Intro(),
					Named:        !block.IsUnnamed(),
					RunAll:       !block.ExcludeFromRunAll(),
				}
				rows = append(rows, r)
			}
			if !formatJSON {
				err := displayTable(io, rows)

				if !io.IsStderrTTY() {
					return err
				}

				_, _ = fmt.Fprintf(io.ErrOut, "\n*) Included when running all via \"run --all\"\n")
				return err
			}

			return displayJSON(io, rows)
		},
	}

	cmd.PersistentFlags().BoolVar(&formatJSON, "json", false, "This flag tells the list command to print the output in json")
	setDefaultFlags(&cmd)

	return &cmd
}

func displayTable(io *iostreams.IOStreams, rows []row) error {
	table := tableprinter.New(io.Out, io.IsStdoutTTY(), io.TerminalWidth())

	// table header
	table.AddField(strings.ToUpper("Name"))
	table.AddField(strings.ToUpper("File"))
	table.AddField(strings.ToUpper("First Command"))
	table.AddField(strings.ToUpper("Description"))
	table.AddField(strings.ToUpper("Named"))
	table.EndRow()

	for _, row := range rows {
		named := "Yes"
		if !row.Named {
			named = "No"
		}
		name := row.Name
		if row.RunAll {
			name += "*"
		}
		table.AddField(name)
		table.AddField(row.File)
		table.AddField(row.FirstCommand)
		table.AddField(row.Description)
		table.AddField(named)
		table.EndRow()
	}

	return errors.Wrap(table.Render(), "failed to render")
}

func displayJSON(io *iostreams.IOStreams, rows []row) error {
	by, err := json.Marshal(&rows)
	if err != nil {
		return err
	}
	return jsonpretty.Format(io.Out, bytes.NewReader(by), "  ", false)
}

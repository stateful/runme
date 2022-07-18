package cmd

import (
	"fmt"

	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/cli/cli/v2/utils"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func listCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List available commands.",
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := newParser()
			if err != nil {
				return errors.Wrap(err, "fail to read README file")
			}

			snippets := p.Snippets()

			// there might be a way to get this from cmd
			io := iostreams.System()
			table := utils.NewTablePrinter(io)

			for _, snippet := range snippets {
				table.AddField(snippet.Name(), nil, nil)
				table.AddField("->", nil, nil)
				table.AddField(snippet.FirstCmd(), nil, nil)
				table.AddField(fmt.Sprintf("[+%d]", len(snippet.Cmds())-1), nil, nil)
				table.AddField(snippet.Description(), nil, nil)
				table.EndRow()
			}

			table.Render()

			return nil
		},
	}
	return &cmd
}

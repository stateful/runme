package cmd

import (
	"fmt"
	"strings"

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
			snippets.PredictLangs()

			// there might be a way to get this from cmd
			io := iostreams.System()
			table := utils.NewTablePrinter(io)

			// table header
			table.AddField(strings.ToUpper("Name"), nil, nil)
			table.AddField(strings.ToUpper("First Command"), nil, nil)
			table.AddField(strings.ToUpper("Count"), nil, nil)
			table.AddField(strings.ToUpper("Description"), nil, nil)
			table.AddField(strings.ToUpper("Language"), nil, nil)
			table.EndRow()

			for _, snippet := range snippets {
				firstCmd := snippet.FirstCmd()

				table.AddField(snippet.Name(), nil, nil)
				table.AddField(firstCmd, nil, nil)
				table.AddField(fmt.Sprintf("%d", len(snippet.Cmds())), nil, nil)
				table.AddField(snippet.Description(), nil, nil)
				table.AddField(snippet.Language(), nil, nil)
				table.EndRow()
			}

			return errors.Wrap(table.Render(), "failed to render")
		},
	}
	return &cmd
}

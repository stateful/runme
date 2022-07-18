package cmd

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func listCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:   "list",
		Short: "List available commands.",
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := newParser()
			if err != nil {
				return errors.Wrap(err, "fail to read README file")
			}

			snippets := p.Snippets()

			for _, snippet := range snippets {
				cmd.Printf("%s -> %s [+%d] (%s)\n", snippet.Name(), snippet.FirstCmd(), len(snippet.Cmds())-1, snippet.Description())
			}

			return nil
		},
	}
	return &cmd
}

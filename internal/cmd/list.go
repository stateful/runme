package cmd

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/stateful/rdme/internal/parser"
)

func listCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:   "list",
		Short: "List available commands.",
		RunE: func(cmd *cobra.Command, args []string) error {
			source, err := os.ReadFile(filepath.Join(chdir, fileName))
			if err != nil {
				return errors.Wrap(err, "fail to read README file")
			}

			p := parser.New(source)
			snippets := p.Snippets()

			for _, snippet := range snippets {
				cmd.Printf("%s -> %s [+%d] (%s)\n", snippet.Name(), snippet.FirstCmd(), len(snippet.Cmds())-1, snippet.Description())
			}

			return nil
		},
	}
	return &cmd
}

package cmd

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/stateful/rdme/internal/parser"
)

var (
	chdir    string
	fileName string
)

func Root() *cobra.Command {
	cmd := cobra.Command{
		Use:           "rdme",
		Short:         "Execute commands directly from a README",
		SilenceErrors: true,
		SilenceUsage:  true,
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
	}

	pflags := cmd.PersistentFlags()

	pflags.StringVar(&chdir, "chdir", ".", "Switch to a different working directory before exeucing the command.")
	pflags.StringVar(&fileName, "filename", "README.md", "A name of the README file.")

	cmd.AddCommand(runCmd())
	cmd.AddCommand(listCmd())
	cmd.AddCommand(tasksCmd())

	return &cmd
}

func newParser() (*parser.Parser, error) {
	source, err := os.ReadFile(filepath.Join(chdir, fileName))
	if err != nil {
		return nil, errors.Wrap(err, "fail to read README file")
	}

	return parser.New(source), nil
}

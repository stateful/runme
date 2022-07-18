package cmd

import (
	"github.com/spf13/cobra"
)

var (
	chdir    string
	fileName string
)

func Root() *cobra.Command {
	cmd := cobra.Command{
		Use:           "mdexec",
		Short:         "Execute commands directly from a README",
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	pflags := cmd.PersistentFlags()

	pflags.StringVar(&chdir, "chdir", ".", "Switch to a different working directory before exeucing the command.")
	pflags.StringVar(&fileName, "filename", "README.md", "A name of the README file.")

	cmd.AddCommand(execCmd())
	cmd.AddCommand(listCmd())
	cmd.AddCommand(tasksCmd())

	return &cmd
}

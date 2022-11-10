package cmd

import (
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	fAllowUnknown bool
	fChdir        string
	fFileName     string
)

func Root() *cobra.Command {
	cmd := cobra.Command{
		Use:           "runme",
		Short:         "Execute commands directly from a README",
		Long:          "Parses commands directly from a README (best-effort) to make them executable under a unique name.",
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if strings.HasPrefix(fChdir, "~") {
				cmd.PrintErrf("WARNING: --chdir starts with ~ which should be resolved by shell. Try re-running with --chdir %s (note lack of =) if it fails.\n\n", fChdir)

				usr, _ := user.Current()

				if fChdir == "~" {
					fChdir = usr.HomeDir
				} else if strings.HasPrefix(fChdir, "~/") {
					fChdir = filepath.Join(usr.HomeDir, fChdir[2:])
				}
			}
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
	}

	setDefaultFlags(&cmd)

	pflags := cmd.PersistentFlags()

	pflags.BoolVar(&fAllowUnknown, "allow-unknown", false, "Display snippets without known executor.")
	pflags.StringVar(&fChdir, "chdir", getCwd(), "Switch to a different working directory before exeucing the command.")
	pflags.StringVar(&fFileName, "filename", "README.md", "A name of the README file.")

	cmd.AddCommand(runCmd())
	cmd.AddCommand(listCmd())
	cmd.AddCommand(printCmd())
	cmd.AddCommand(tasksCmd())
	cmd.AddCommand(fmtCmd())
	cmd.AddCommand(daemonCmd())
	cmd.AddCommand(shellCmd())

	return &cmd
}

func getCwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	return cwd
}

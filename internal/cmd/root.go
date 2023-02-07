package cmd

import (
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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

	setConfigFlags(pflags)

	tuiCmd := tuiCmd()
	// Make tuiCmd a default command.
	cmd.RunE = tuiCmd.RunE

	tuiCmd.Flags().VisitAll(func(f *pflag.Flag) {
		if flag := cmd.Flags().Lookup(f.Name); flag == nil {
			cmd.Flags().AddFlag(f)
		}
	})

	cmd.AddCommand(tuiCmd)
	cmd.AddCommand(runCmd())
	cmd.AddCommand(listCmd())
	cmd.AddCommand(printCmd())
	cmd.AddCommand(tasksCmd())
	cmd.AddCommand(fmtCmd())
	cmd.AddCommand(serverCmd())
	cmd.AddCommand(shellCmd())
	cmd.AddCommand(authCmd())
	cmd.AddCommand(suggestCmd())

	return &cmd
}

func getCwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	return cwd
}

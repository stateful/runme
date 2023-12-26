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
	fAllowUnknown          bool
	fAllowUnnamed          bool
	fChdir                 string
	fFileName              string
	fFileMode              bool
	fProject               string
	fProjectIgnorePatterns []string
	fRespectGitignore      bool
	fInsecure              bool
)

func Root() *cobra.Command {
	cmd := cobra.Command{
		Use:           "runme",
		Short:         "Execute commands directly from a README",
		Long:          "Runme executes commands inside your runbooks, docs, and READMEs. Parses commands\ndirectly from markdown files to make them executable.",
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

			// backwards compat
			if envProject, ok := os.LookupEnv("RUNME_PROJECT"); ok {
				fProject = envProject
			}

			fFileMode = cmd.Flags().Changed("chdir") || cmd.Flags().Changed("filename")

			if fFileMode && !cmd.Flags().Changed("allow-unnamed") {
				fAllowUnnamed = true
			}
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
	}

	setDefaultFlags(&cmd)

	pflags := cmd.PersistentFlags()

	pflags.BoolVar(&fAllowUnknown, "allow-unknown", true, "Display snippets without known executor")
	pflags.BoolVar(&fAllowUnnamed, "allow-unnamed", false, "Allow scripts without explicit names")

	pflags.StringVar(&fChdir, "chdir", getCwd(), "Switch to a different working directory before executing the command")
	pflags.StringVar(&fFileName, "filename", "README.md", "Name of the README file")
	pflags.BoolVar(&fInsecure, "insecure", false, "Run command in insecure-mode")

	pflags.StringVar(&fProject, "project", ".", "Root project to find runnable tasks")
	pflags.BoolVar(&fRespectGitignore, "git-ignore", true, "Whether to respect .gitignore file(s) in project")
	pflags.StringArrayVar(&fProjectIgnorePatterns, "ignore-pattern", []string{"node_modules"}, "Patterns to ignore in project mode")

	setAPIFlags(pflags)

	tuiCmd := tuiCmd()
	// Make tuiCmd a default command.
	cmd.RunE = tuiCmd.RunE

	tuiCmd.Flags().VisitAll(func(f *pflag.Flag) {
		if flag := cmd.Flags().Lookup(f.Name); flag == nil {
			cmd.Flags().AddFlag(f)
		}
	})

	branchCmd := branchCmd()
	suggestCmd := suggestCmd()
	suggestCmd.AddCommand(branchCmd)

	cmd.AddCommand(branchCmd)
	cmd.AddCommand(codeServerCmd())
	cmd.AddCommand(environmentCmd())
	cmd.AddCommand(fmtCmd())
	cmd.AddCommand(listCmd())
	cmd.AddCommand(loginCmd())
	cmd.AddCommand(logoutCmd())
	cmd.AddCommand(printCmd())
	cmd.AddCommand(extensionCmd())
	cmd.AddCommand(runCmd())
	cmd.AddCommand(serverCmd())
	cmd.AddCommand(shellCmd())
	cmd.AddCommand(suggestCmd)
	cmd.AddCommand(tasksCmd())
	cmd.AddCommand(tokenCmd())
	cmd.AddCommand(tuiCmd)

	cmd.SetUsageTemplate(getUsageTemplate(cmd))

	return &cmd
}

func getUsageTemplate(cmd cobra.Command) string {
	return cmd.UsageTemplate() + `
Feedback:
  For issues and questions join the Runme community at https://discord.gg/runme
`
}

func getCwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	return cwd
}

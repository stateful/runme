package cmd

import (
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/stateful/runme/v3/internal/cmd/beta"
	"github.com/stateful/runme/v3/internal/extension"
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
	fSkipRunnerFallback    bool
	fInsecure              bool
	fLogEnabled            bool
	fLogFilePath           string
	fExtensionHandle       string
	fStateful              bool
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

			// if insecure is explicitly set irrespective of its value, skip runner fallback
			fSkipRunnerFallback = !cmd.Flags().Changed("insecure")

			fFileMode = cmd.Flags().Changed("chdir") || cmd.Flags().Changed("filename")

			if fFileMode && !cmd.Flags().Changed("allow-unnamed") {
				fAllowUnnamed = true
			}

			if fExtensionHandle == "" && !fStateful {
				fExtensionHandle = extension.DefaultExtensionName
			} else {
				fExtensionHandle = extension.PlatformExtensionName
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
	pflags.BoolVar(&fInsecure, "insecure", false, "Explicitly allow insecure operations to prevent misuse")

	pflags.StringVar(&fProject, "project", "", "Root project to find runnable tasks")
	pflags.BoolVar(&fRespectGitignore, "git-ignore", true, "Whether to respect .gitignore file(s) in project")
	pflags.StringArrayVar(&fProjectIgnorePatterns, "ignore-pattern", []string{"node_modules", ".venv"}, "Patterns to ignore in project mode")

	pflags.BoolVar(&fLogEnabled, "log", false, "Enable logging")
	pflags.StringVar(&fLogFilePath, "log-file", filepath.Join(getTempDir(), "runme.log"), "Log file path")

	pflags.BoolVar(&fStateful, "stateful", false, "Set Stateful instead of the Runme default")

	setAPIFlags(pflags)

	tuiCmd := tuiCmd()
	// Make tuiCmd a default command.
	cmd.RunE = tuiCmd.RunE

	tuiCmd.Flags().VisitAll(func(f *pflag.Flag) {
		if flag := cmd.Flags().Lookup(f.Name); flag == nil {
			cmd.Flags().AddFlag(f)
		}
	})

	cmd.AddCommand(codeServerCmd())
	cmd.AddCommand(environmentCmd())
	cmd.AddCommand(fmtCmd())
	cmd.AddCommand(listCmd())
	cmd.AddCommand(loginCmd())
	cmd.AddCommand(logoutCmd())
	cmd.AddCommand(printCmd())
	cmd.AddCommand(extensionCmd())
	cmd.AddCommand(runCmd())
	cmd.AddCommand(beta.BetaCmd())
	cmd.AddCommand(serverCmd())
	cmd.AddCommand(shellCmd())
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

func getTempDir() string {
	tmp := "/var/tmp"
	if _, err := os.Stat(tmp); err != nil {
		tmp = os.TempDir()
	}
	return tmp
}

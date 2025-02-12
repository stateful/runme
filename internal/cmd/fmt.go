package cmd

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	idt "github.com/stateful/runme/v3/pkg/document/identity"
	"github.com/stateful/runme/v3/pkg/project"
)

var (
	flatten     bool
	formatJSON  bool
	identityStr string
	write       bool
)

func buildFmtCmd(cmd *cobra.Command, reset bool) *cobra.Command {
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if formatJSON {
			if write {
				return errors.New("invalid usage of --json with --write")
			}
			if !flatten {
				return errors.New("invalid usage of --json without --flatten")
			}
		}

		files := args

		if len(args) == 0 {
			var err error
			files, err = getProjectFiles(cmd)
			if err != nil {
				return err
			}
		}

		identityResolver := idt.NewResolver(idt.UnspecifiedLifecycleIdentity)
		if reset {
			identityResolver = strToIdentityResolver(identityStr)
		}

		return project.FormatFiles(files, &project.FormatOptions{
			FormatJSON:       formatJSON,
			IdentityResolver: identityResolver,
			Outputter: func(file string, formatted []byte) error {
				out := cmd.OutOrStdout()
				_, _ = fmt.Fprintf(out, "===== %s =====\n", file)
				_, _ = out.Write(formatted)
				_, _ = fmt.Fprint(out, "\n")
				return nil
			},
			Reset: reset,
			Write: write,
		})
	}
	setDefaultFlags(cmd)

	cmd.Flags().BoolVar(&flatten, "flatten", true, "Flatten nested blocks in the output. WARNING: This can currently break frontmatter if turned off.")
	cmd.Flags().BoolVar(&formatJSON, "json", false, "Print out data as JSON. Only possible with --flatten and not allowed with --write.")
	cmd.Flags().BoolVarP(&write, "write", "w", false, "Write result to the source file instead of stdout.")
	cmd.Flags().StringVar(&identityStr, "identity", "", "Set the lifecycle identity, \"doc\", \"cell\", \"all\", or \"\" (default).")
	_ = cmd.Flags().MarkDeprecated("flatten", "This flag is now default and no longer has any other effect.")

	return cmd
}

func fmtCmd() *cobra.Command {
	cmd := buildFmtCmd(&cobra.Command{
		Use:   "fmt",
		Short: "Format a Markdown file into canonical format",
		Args:  cobra.MaximumNArgs(1),
	}, false)

	cmd.AddCommand(buildFmtCmd(&cobra.Command{
		Use:   "reset",
		Short: "Format a Markdown file and reset all lifecycle metadata",
		Args:  cobra.MaximumNArgs(1),
	}, true))

	return cmd
}

func strToIdentityResolver(identity string) *idt.IdentityResolver {
	var identityResolver *idt.IdentityResolver
	switch strings.ToLower(identity) {
	case "all":
		identityResolver = idt.NewResolver(idt.AllLifecycleIdentity)
	case "doc", "document":
		identityResolver = idt.NewResolver(idt.DocumentLifecycleIdentity)
	case "cell":
		identityResolver = idt.NewResolver(idt.CellLifecycleIdentity)
	default:
		identityResolver = idt.NewResolver(idt.DefaultLifecycleIdentity)
	}
	return identityResolver
}

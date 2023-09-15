package cmd

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/stateful/runme/internal/project"
)

func fmtCmd() *cobra.Command {
	var (
		formatJSON bool
		flatten    bool
		write      bool
	)

	cmd := cobra.Command{
		Use:   "fmt",
		Short: "Format a Markdown file into canonical format",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if formatJSON {
				if write {
					return errors.New("invalid usage of --json with --write")
				}
				if !flatten {
					return errors.New("invalid usage of --json without --flatten")
				}
			}

			proj, err := getProject()
			if err != nil {
				return err
			}

			files := args

			if len(files) == 0 {
				projectFiles, err := loadFiles(proj, cmd.OutOrStdout(), cmd.InOrStdin())
				if err != nil {
					return err
				}

				files = projectFiles
			}

			err = project.Format(files, proj.Dir(), flatten, formatJSON, write, func(file string, formatted []byte) error {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "===== %s =====\n", file)
				_, _ = cmd.OutOrStdout().Write(formatted)
				_, _ = fmt.Fprint(cmd.OutOrStdout(), "\n")
				return nil
			})
			if err != nil {
				return err
			}

			return nil
		},
	}

	setDefaultFlags(&cmd)

	cmd.Flags().BoolVar(&flatten, "flatten", true, "Flatten nested blocks in the output. WARNING: This can currently break frontmatter if turned off.")
	cmd.Flags().BoolVar(&formatJSON, "json", false, "Print out data as JSON. Only possible with --flatten and not allowed with --write.")
	cmd.Flags().BoolVarP(&write, "write", "w", false, "Write result to the source file instead of stdout.")

	return &cmd
}

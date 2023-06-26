package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/stateful/runme/internal/document"
	"github.com/stateful/runme/internal/document/editor"
	"github.com/stateful/runme/internal/renderer/cmark"
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

			for _, relFile := range files {
				mdFilePath := filepath.Join(proj.Dir(), relFile)

				data, err := readMarkdownFile([]string{mdFilePath})
				if err != nil {
					return err
				}

				var formatted []byte

				if flatten {
					notebook, err := editor.Deserialize(data)
					if err != nil {
						return errors.Wrap(err, "failed to deserialize")
					}

					if formatJSON {
						var buf bytes.Buffer
						enc := json.NewEncoder(&buf)
						enc.SetIndent("", "  ")
						if err := enc.Encode(notebook); err != nil {
							return errors.Wrap(err, "failed to encode to JSON")
						}
						formatted = buf.Bytes()
					} else {
						formatted, err = editor.Serialize(notebook)
						if err != nil {
							return errors.Wrap(err, "failed to serialize")
						}
					}
				} else {
					doc := document.New(data, cmark.Render)
					_, astNode, err := doc.Parse()
					if err != nil {
						return errors.Wrap(err, "failed to parse source")
					}
					formatted, err = cmark.Render(astNode, data)
					if err != nil {
						return errors.Wrap(err, "failed to render")
					}
				}

				if write {
					err = writeMarkdownFile([]string{mdFilePath}, formatted)
				} else {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "===== %s =====\n", relFile)
					_, _ = cmd.OutOrStdout().Write(formatted)
					_, _ = fmt.Fprint(cmd.OutOrStdout(), "\n")
				}

				if err != nil {
					return err
				}
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

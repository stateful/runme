package cmd

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/stateful/runme/internal/document"
	"github.com/stateful/runme/internal/document/edit"
	"github.com/stateful/runme/internal/renderer/cmark"
)

func fmtCmd() *cobra.Command {
	var (
		flatten bool
		write   bool
	)

	cmd := cobra.Command{
		Use:   "fmt",
		Short: "Format a Markdown file into canonical format.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := readMarkdownFile(args)
			if err != nil {
				return err
			}

			var formatted []byte

			if flatten {
				editor := edit.New()
				notebook, err := editor.Deserialize(data)
				if err != nil {
					return errors.Wrap(err, "failed to deserialize")
				}
				formatted, err = editor.Serialize(notebook)
				if err != nil {
					return errors.Wrap(err, "failed to serialize")
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
				err = writeMarkdownFile(args, formatted)
			} else {
				_, err = cmd.OutOrStdout().Write(formatted)
				if err != nil {
					err = errors.Wrap(err, "failed to write out result")
				}
			}
			return err
		},
	}

	cmd.Flags().BoolVar(&flatten, "flatten", false, "Flatten nested blocks in the output.")
	cmd.Flags().BoolVarP(&write, "write", "w", false, "Write result to the source file instead of stdout.")

	return &cmd
}

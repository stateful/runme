package cmd

import (
	"io"
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/stateful/runme/internal/document"
	"github.com/stateful/runme/internal/renderer/md"
)

func fmtCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:    "fmt",
		Short:  "Format a Markdown file into canonical format. Caution, this is experimental.",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var data []byte

			fileName := args[0]
			if fileName == "-" {
				var err error
				data, err = io.ReadAll(os.Stdin)
				if err != nil {
					return errors.Wrap(err, "failed to read from stdin")
				}
			} else {
				f, err := os.Open(fileName)
				if err != nil {
					return errors.Wrapf(err, "failed to open file %q", fileName)
				}
				data, err = io.ReadAll(f)
				if err != nil {
					return errors.Wrapf(err, "failed to read from file %q", fileName)
				}
			}

			result, err := md.Render(document.NewSource(data).Parse().Root(), data)
			if err != nil {
				return errors.Wrap(err, "failed to format source")
			}

			_, err = cmd.OutOrStdout().Write(result)
			return errors.Wrap(err, "failed to write result")
		},
	}
	return &cmd
}

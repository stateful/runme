package cmd

import (
	"io"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func printCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:               "print",
		Short:             "Print a selected snippet.",
		Long:              "Print will display the details of the corresponding command block based on its name.",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: validCmdNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := newParser()
			if err != nil {
				return err
			}

			snippet, err := lookup(p.Snippets(), args[0])
			if err != nil {
				return err
			}

			w := &bulkWriter{Writer: cmd.OutOrStdout()}

			_, _ = w.Write([]byte(snippet.GetContent()))
			_, err = w.Write([]byte{'\n'})
			return errors.Wrap(err, "failed to write to stdout")
		},
	}

	return &cmd
}

type bulkWriter struct {
	io.Writer
	err error
}

func (w *bulkWriter) Write(p []byte) (n int, err error) {
	if w.err != nil {
		return 0, err
	}
	return w.Writer.Write(p)
}

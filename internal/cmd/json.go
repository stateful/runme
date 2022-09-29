package cmd

import (
	"bytes"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func jsonCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:    "json",
		Short:  "Generates json. Caution, this is experimental.",
		Hidden: true,
		Args:   cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := newParser()
			if err != nil {
				return err
			}

			var b bytes.Buffer
			err = p.Render(&b)
			if err != nil {
				return errors.Wrapf(err, "error rendering")
			}

			cmd.Printf("%s\n", b.String())
			return nil
		},
	}
	return &cmd
}

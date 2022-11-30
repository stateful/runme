package cmd

import (
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/stateful/runme/internal/document/edit"
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
			} else if strings.HasPrefix(fileName, "https://") {
				client := http.Client{
					Timeout: time.Second * 10,
				}
				resp, err := client.Get(fileName)
				if err != nil {
					return errors.Wrapf(err, "failed to get a file %q", fileName)
				}
				data, err = io.ReadAll(resp.Body)
				_ = resp.Body.Close()
				if err != nil {
					return errors.Wrap(err, "failed to read body")
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

			editor := edit.New()
			cells, err := editor.Deserialize(data)
			if err != nil {
				return errors.Wrap(err, "failed to deserialize source")
			}
			result, err := editor.Serialize(cells)
			if err != nil {
				return errors.Wrap(err, "failed to serialize cells")
			}
			_, err = cmd.OutOrStdout().Write(result)
			return errors.Wrap(err, "failed to write result")
		},
	}
	return &cmd
}

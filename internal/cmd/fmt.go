package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/stateful/runme/v3/internal/renderer/cmark"
	"github.com/stateful/runme/v3/pkg/document"
	"github.com/stateful/runme/v3/pkg/document/editor"
	"github.com/stateful/runme/v3/pkg/document/identity"
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

			files := args

			if len(args) == 0 {
				var err error
				files, err = getProjectFiles(cmd)
				if err != nil {
					return err
				}
			}

			return fmtFiles(files, flatten, formatJSON, write, func(file string, formatted []byte) error {
				out := cmd.OutOrStdout()
				_, _ = fmt.Fprintf(out, "===== %s =====\n", file)
				_, _ = out.Write(formatted)
				_, _ = fmt.Fprint(out, "\n")
				return nil
			})
		},
	}

	setDefaultFlags(&cmd)

	cmd.Flags().BoolVar(&flatten, "flatten", true, "Flatten nested blocks in the output. WARNING: This can currently break frontmatter if turned off.")
	cmd.Flags().BoolVar(&formatJSON, "json", false, "Print out data as JSON. Only possible with --flatten and not allowed with --write.")
	cmd.Flags().BoolVarP(&write, "write", "w", false, "Write result to the source file instead of stdout.")

	return &cmd
}

type funcOutput func(string, []byte) error

func fmtFiles(files []string, flatten bool, formatJSON bool, write bool, outputter funcOutput) error {
	logger, err := getLogger(false, false)
	if err != nil {
		return err
	}
	identityResolver := identity.NewResolver(identity.DefaultLifecycleIdentity)

	for _, file := range files {
		data, err := readMarkdown(file)
		if err != nil {
			return err
		}

		var formatted []byte

		if flatten {
			notebook, err := editor.Deserialize(logger, data, identityResolver)
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
				formatted, err = editor.Serialize(logger, notebook, nil)
				if err != nil {
					return errors.Wrap(err, "failed to serialize")
				}
			}
		} else {
			doc := document.New(data, identityResolver)
			astNode, err := doc.RootAST()
			if err != nil {
				return errors.Wrap(err, "failed to parse source")
			}
			formatted, err = cmark.Render(astNode, data)
			if err != nil {
				return errors.Wrap(err, "failed to render")
			}
		}

		if write {
			err = writeMarkdown(file, formatted)
		} else {
			err = outputter(file, formatted)
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func readMarkdown(source string) ([]byte, error) {
	var (
		data []byte
		err  error
	)

	if source == "-" {
		data, err = io.ReadAll(os.Stdin)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read from stdin")
		}
	} else if strings.HasPrefix(source, "https://") {
		client := http.Client{
			Timeout: time.Second * 5,
		}
		resp, err := client.Get(source)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get a file %q", source)
		}
		defer func() { _ = resp.Body.Close() }()
		data, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read body")
		}
	} else {
		data, err = os.ReadFile(source)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read from file %q", source)
		}
	}

	return data, nil
}

func writeMarkdown(destination string, data []byte) error {
	if destination == "-" {
		_, err := os.Stdout.Write(data)
		return errors.Wrap(err, "failed to write to stdout")
	}
	if strings.HasPrefix(destination, "https://") {
		return errors.New("cannot write to HTTPS location")
	}
	err := os.WriteFile(destination, data, 0o600)
	return errors.Wrapf(err, "failed to write data to %q", destination)
}

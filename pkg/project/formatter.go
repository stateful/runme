package project

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/stateful/runme/v3/internal/document"
	"github.com/stateful/runme/v3/internal/document/editor"
	"github.com/stateful/runme/v3/internal/document/identity"
	"github.com/stateful/runme/v3/internal/renderer/cmark"
)

type funcOutput func(string, []byte) error

func Format(files []string, basePath string, flatten bool, formatJSON bool, write bool, outputter funcOutput) error {
	for _, relFile := range files {
		data, err := readMarkdown(basePath, []string{relFile})
		if err != nil {
			return err
		}

		var formatted []byte
		identityResolver := identity.NewResolver(identity.DefaultLifecycleIdentity)

		if flatten {
			notebook, err := editor.Deserialize(data, identityResolver)
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
				if identityResolver.CellEnabled() {
					notebook.ForceLifecycleIdentities()
				}

				formatted, err = editor.Serialize(notebook, nil)
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
			err = writeMarkdown(basePath, []string{relFile}, formatted)
		} else {
			err = outputter(relFile, formatted)
		}

		if err != nil {
			return err
		}
	}

	return nil
}

func writeMarkdown(basePath string, args []string, data []byte) error {
	arg := ""
	if len(args) == 1 {
		arg = args[0]
	}

	if arg == "-" {
		return errors.New("cannot write to stdin")
	}

	if strings.HasPrefix(arg, "https://") {
		return errors.New("cannot write to HTTP location")
	}

	fullFilename := filepath.Join(basePath, arg)
	if fullFilename == "" {
		return nil
	}
	err := WriteMarkdownFile(fullFilename, nil, data)
	return errors.Wrapf(err, "failed to write to %s", fullFilename)
}

func readMarkdown(basePath string, args []string) ([]byte, error) {
	arg := ""
	if len(args) == 1 {
		arg = args[0]
	}

	var (
		data []byte
		err  error
	)

	if arg == "-" {
		data, err = io.ReadAll(os.Stdin)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read from stdin")
		}
	} else if strings.HasPrefix(arg, "https://") {
		client := http.Client{
			Timeout: time.Second * 5,
		}
		resp, err := client.Get(arg)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get a file %q", arg)
		}
		defer func() { _ = resp.Body.Close() }()
		data, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read body")
		}
	} else {
		filePath := filepath.Join(basePath, arg)
		data, err = ReadMarkdownFile(filePath, nil)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read from file %q", arg)
		}
	}

	return data, nil
}

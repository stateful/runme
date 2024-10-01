package project

import (
	"bytes"
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/stateful/runme/v3/pkg/document/editor"
	"github.com/stateful/runme/v3/pkg/document/identity"
)

type FuncOutput func(string, []byte) error

func FormatFiles(files []string, identityResolver *identity.IdentityResolver, formatJSON bool, write bool, outputter FuncOutput) error {
	for _, file := range files {
		data, err := readMarkdown(file)
		if err != nil {
			return err
		}

		formatted, err := formatFile(data, identityResolver, formatJSON)
		if err != nil {
			return err
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

func formatFile(data []byte, identityResolver *identity.IdentityResolver, formatJSON bool) ([]byte, error) {
	var formatted []byte

	notebook, err := editor.Deserialize(data, editor.Options{IdentityResolver: identityResolver})
	if err != nil {
		return nil, errors.Wrap(err, "failed to deserialize")
	}

	if formatJSON {
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetIndent("", "  ")
		if err := enc.Encode(notebook); err != nil {
			return nil, errors.Wrap(err, "failed to encode to JSON")
		}
		formatted = buf.Bytes()
	} else {
		formatted, err = editor.Serialize(notebook, nil, editor.Options{IdentityResolver: identityResolver})
		if err != nil {
			return nil, errors.Wrap(err, "failed to serialize")
		}
	}

	// todo(sebastian): remove moving to beta? it's neither used nor maintained
	// {
	// 	doc := document.New(data, identityResolver)
	// 	astNode, err := doc.RootAST()
	// 	if err != nil {
	// 		return nil, errors.Wrap(err, "failed to parse source")
	// 	}
	// 	formatted, err = cmark.Render(astNode, data)
	// 	if err != nil {
	// 		return nil, errors.Wrap(err, "failed to render")
	// 	}
	// }

	return formatted, nil
}

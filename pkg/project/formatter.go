package project

import (
	"bytes"
	"encoding/json"

	"github.com/pkg/errors"

	"github.com/runmedev/runme/v3/pkg/document/editor"
	"github.com/runmedev/runme/v3/pkg/document/identity"
)

type FuncOutput func(string, []byte) error

type FormatOptions struct {
	IdentityResolver *identity.IdentityResolver
	FormatJSON       bool
	Write            bool
	Outputter        FuncOutput
	Reset            bool
}

func FormatFiles(files []string, options *FormatOptions) error {
	for _, file := range files {
		data, err := readMarkdown(file)
		if err != nil {
			return err
		}

		formatted, err := formatFile(data, options)
		if err != nil {
			return errors.Wrapf(err, "failed to format %s", file)
		}

		if options.Write {
			err = writeMarkdown(file, formatted)
		} else if outputter := options.Outputter; outputter != nil {
			err = outputter(file, formatted)
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func formatFile(data []byte, options *FormatOptions) ([]byte, error) {
	var formatted []byte

	notebook, err := editor.Deserialize(data, editor.Options{IdentityResolver: options.IdentityResolver, Reset: options.Reset})
	if err != nil {
		return nil, errors.Wrap(err, "failed to deserialize")
	}

	if options.FormatJSON {
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetIndent("", "  ")
		if err := enc.Encode(notebook); err != nil {
			return nil, errors.Wrap(err, "failed to encode to JSON")
		}
		formatted = buf.Bytes()
	} else {
		formatted, err = editor.Serialize(notebook, nil, editor.Options{IdentityResolver: options.IdentityResolver})
		if err != nil {
			return nil, errors.Wrap(err, "failed to serialize")
		}
	}

	// todo(sebastian): remove moving to beta? it's neither used nor maintained
	// {
	// 	doc := document.New(data, options.IdentityResolver)
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

package renderer

import (
	"encoding/json"
	"io"

	"github.com/pkg/errors"
	"github.com/stateful/runme/internal/document"
)

func ToJSON(parsed *document.ParsedSource, w io.Writer) error {
	blocks := parsed.Blocks()

	type wrapper struct {
		Cells document.Blocks `json:"cells"`
	}

	jsonDoc, err := json.Marshal(&wrapper{Cells: blocks})
	if err != nil {
		return errors.WithMessage(err, "error marshaling json")
	}

	_, err = w.Write(jsonDoc)
	return errors.WithMessage(err, "error writing json")
}

func ToNotebookData(parsed *document.ParsedSource, w io.Writer) error {
	blocks := parsed.Blocks()

	cells := make([]document.Cell, 0, len(blocks))
	for _, block := range blocks {
		switch b := block.(type) {
		case *document.MarkdownBlock:
			cells = append(cells, document.Cell{
				Kind:  document.MarkupKind,
				Value: b.Content(),
			})
		case *document.CodeBlock:
			metadata := make(map[string]any)
			for k, v := range b.Attributes() {
				metadata[k] = v
			}
			metadata["name"] = b.Name()

			cells = append(cells, document.Cell{
				Kind:     document.CodeKind,
				LangID:   b.Executable(),
				Value:    b.Content(),
				Metadata: metadata,
			})
		}
	}

	enc := json.NewEncoder(w)
	if err := enc.Encode(&document.Notebook{Cells: cells}); err != nil {
		return errors.WithMessage(err, "error marshaling json")
	}
	return nil
}

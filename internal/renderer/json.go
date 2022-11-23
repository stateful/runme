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
	cells := document.BlocksToCells(blocks)
	enc := json.NewEncoder(w)
	if err := enc.Encode(&document.Notebook{Cells: cells}); err != nil {
		return errors.WithMessage(err, "error marshaling json")
	}
	return nil
}

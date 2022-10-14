package renderer

import (
	"encoding/json"
	"io"

	"github.com/pkg/errors"
	"github.com/stateful/runme/internal/document"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	goldrender "github.com/yuin/goldmark/renderer"
)

type jsonRenderer struct{}

func (r *jsonRenderer) Render(w io.Writer, source []byte, n ast.Node) error {
	blocks, err := document.NewSource(source).Parse().SquashedBlocks()
	if err != nil {
		return errors.WithMessage(err, "failed to parse source")
	}

	type wrapper struct {
		Document document.Blocks `json:"document,omitempty"`
	}

	jsonDoc, err := json.Marshal(&wrapper{Document: blocks})
	if err != nil {
		return errors.WithMessage(err, "error marshaling json")
	}

	_, err = w.Write(jsonDoc)
	return errors.WithMessage(err, "error writing json")
}

// AddOptions has no effect
func (r *jsonRenderer) AddOptions(_ ...goldrender.Option) {
	// Nothing to do here
}

func RenderToJSON(w io.Writer, source []byte, root ast.Node) error {
	mdr := goldmark.New(goldmark.WithRenderer(&jsonRenderer{}))
	err := mdr.Renderer().Render(w, source, root)
	return errors.WithStack(err)
}

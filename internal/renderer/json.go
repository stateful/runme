package renderer

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/pkg/errors"
	"github.com/yuin/goldmark/ast"
	goldrender "github.com/yuin/goldmark/renderer"
)

type renderer struct {
	source []byte
}

func NewJSON(source []byte, options ...goldrender.Option) goldrender.Renderer {
	r := &renderer{source}

	return r
}

func (r *renderer) Render(w io.Writer, source []byte, n ast.Node) error {
	segments := []map[string]interface{}{}
	lastCodeBlock := &n
	remainingNode := n

	err := ast.Walk(n, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		s := ast.WalkStatus(ast.WalkContinue)

		if n.Kind() != ast.KindFencedCodeBlock || !entering {
			return s, nil
		}

		start := r.getMarkdownStart(n)

		if lastCodeBlock != nil {
			prevStop := r.getMarkdownStop(*lastCodeBlock)
			mdStr := string(source[prevStop:start])
			seg := map[string]interface{}{"Markdown": mdStr}
			segments = append(segments, seg)
			lastCodeBlock = &n
		}

		codeBlock := n.(*ast.FencedCodeBlock)
		seg := map[string]interface{}{}
		seg["Code"] = r.getContent(n)
		seg["Language"] = string(codeBlock.Language(source))
		seg["Description"] = r.getContent(n.PreviousSibling())
		segments = append(segments, seg)

		remainingNode = n.NextSibling()

		return s, nil
	})
	if err != nil {
		return err
	}

	if remainingNode != nil {
		start := remainingNode.Lines().At(0).Start
		stop := len(source) - 1
		seg := map[string]interface{}{"Markdown": string(r.source[start:stop])}
		segments = append(segments, seg)
	}

	doc := map[string]interface{}{"Document": segments}

	jsonDoc, err := json.Marshal(doc)
	if err != nil {
		return errors.Wrapf(err, "error marshaling json doc")
	}

	_, err = w.Write(jsonDoc)
	if err != nil {
		return errors.Wrapf(err, "error writing json doc")
	}

	return nil
}

func (r *renderer) getContent(n ast.Node) string {
	var content strings.Builder
	switch n.Type() {
	case ast.TypeInline:
		content.Write(n.Text(r.source))
	default:
		for i := 0; i < n.Lines().Len(); i++ {
			line := n.Lines().At(i)
			_, _ = content.Write(r.source[line.Start:line.Stop])
		}
	}
	return content.String()
}

func (r *renderer) getMarkdownStart(n ast.Node) int {
	c := n.PreviousSibling()
	return c.Lines().At(0).Stop
}

func (r *renderer) getMarkdownStop(n ast.Node) int {
	curr := n
	next := n.NextSibling()
	if next != nil {
		curr = next
	}

	l := curr.Lines().Len()
	if l == 0 {
		return 0
	}

	return curr.Lines().At(l - 1).Start
}

// AddOptions has no effect
func (r *renderer) AddOptions(_ ...goldrender.Option) {
	// Nothing to do here
}

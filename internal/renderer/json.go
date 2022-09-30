package renderer

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/pkg/errors"
	"github.com/stateful/rdme/internal/snippets"
	"github.com/yuin/goldmark/ast"
	goldrender "github.com/yuin/goldmark/renderer"
)

type renderer struct {
	source   []byte
	rootNode ast.Node
}

type document struct {
	Document snippets.Snippets `json:"document,omitempty"`
}

func NewJSON(source []byte, rootNode ast.Node, options ...goldrender.Option) goldrender.Renderer {
	r := &renderer{source, rootNode}

	return r
}

func (r *renderer) GetDocument(source []byte, n ast.Node) (document, error) {
	var snips snippets.Snippets
	lastCodeBlock := n
	remainingNode := n

	err := ast.Walk(n, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		s := ast.WalkStatus(ast.WalkContinue)

		if n.Kind() != ast.KindFencedCodeBlock || !entering {
			return s, nil
		}

		start := r.getMarkdownStart(n)

		if lastCodeBlock != nil {
			prevStop := r.getMarkdownStop(lastCodeBlock)
			md := string(source[prevStop:start])
			snip := snippets.Snippet{
				Markdown: md,
			}
			snips = append(snips, &snip)

			lastCodeBlock = n
		}

		codeBlock := n.(*ast.FencedCodeBlock)
		snip := snippets.Snippet{
			Attributes:  snippets.ParseAttributes(snippets.ExtractRawAttributes(r.source, codeBlock)),
			Content:     r.getContent(n),
			Description: r.getContent(n.PreviousSibling()),
			Language:    string(codeBlock.Language(source)),
		}
		snip.Lines = snip.GetLines() // ugly hack
		snips = append(snips, &snip)

		remainingNode = n.NextSibling()

		return s, nil
	})
	if err != nil {
		return document{}, err
	}

	if remainingNode != nil {
		start := remainingNode.Lines().At(0).Start
		stop := len(source) - 1
		snip := snippets.Snippet{
			Markdown: string(r.source[start:stop]),
		}
		snips = append(snips, &snip)
	}

	doc := document{
		Document: snips,
	}

	return doc, nil
}

func (r *renderer) Render(w io.Writer, source []byte, n ast.Node) error {
	doc, err := r.GetDocument(source, r.rootNode)
	if err != nil {
		return errors.Wrapf(err, "error processing ast")
	}

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

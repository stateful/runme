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

		start := r.getPrevStart(n)

		if lastCodeBlock != nil {
			prevStop := r.getNextStop(lastCodeBlock)
			// check for existence of markdown in between code blocks
			if start > prevStop {
				md := string(source[prevStop:start])
				snip := snippets.Snippet{
					Markdown: md,
				}
				snips = append(snips, &snip)
			}
			lastCodeBlock = n
		}

		nContent, err := r.getContent(n)
		if err != nil {
			return s, errors.Wrapf(err, "error getting content")
		}
		pnContent, err := r.getContent(n.PreviousSibling())
		if err != nil {
			return s, errors.Wrapf(err, "error getting content")
		}

		codeBlock := n.(*ast.FencedCodeBlock)
		snip := snippets.Snippet{
			Attributes:  snippets.ParseAttributes(snippets.ExtractRawAttributes(r.source, codeBlock)),
			Content:     nContent,
			Description: pnContent,
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

	// Never encounter a code block, stuck on document node
	if remainingNode == n {
		remainingNode = remainingNode.FirstChild()
	}

	// Skip remainingNodes unless it's got lines
	for remainingNode != nil && remainingNode.Lines().Len() == 0 {
		remainingNode = remainingNode.NextSibling()
	}

	if remainingNode != nil {
		start := remainingNode.Lines().At(0).Start
		stop := len(source) - 1
		snip := snippets.Snippet{
			Markdown: string(r.source[start:stop]),
		}
		snips = append(snips, &snip)
	}

	if len(snips) == 2 && snips[0].Content == "" {
		snips = snips[1:]
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

func (r *renderer) getContent(n ast.Node) (string, error) {
	if n == nil {
		return "", nil
	}
	var content strings.Builder
	switch n.Type() {
	case ast.TypeInline:
		_, err := content.Write(n.Text(r.source))
		return "", err
	default:
		for i := 0; i < n.Lines().Len(); i++ {
			line := n.Lines().At(i)
			_, _ = content.Write(r.source[line.Start:line.Stop])
		}
	}
	return content.String(), nil
}

func (r *renderer) getPrevStart(n ast.Node) int {
	curr := n
	prev := n.PreviousSibling()
	if prev != nil {
		curr = prev
	}
	return curr.Lines().At(0).Stop
}

func (r *renderer) getNextStop(n ast.Node) int {
	curr := n
	next := n.NextSibling()
	if next != nil {
		curr = next
	}

	l := curr.Lines().Len()
	if l == 0 {
		return 0
	}

	stop := curr.Lines().At(l - 1).Start

	// add back markdown heading levels
	if curr.Kind() == ast.KindHeading {
		heading := curr.(*ast.Heading)
		// simple math to add back ## incl trailing space
		stop = stop - 1 - heading.Level
	}

	return stop
}

// AddOptions has no effect
func (r *renderer) AddOptions(_ ...goldrender.Option) {
	// Nothing to do here
}

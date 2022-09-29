package renderer

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/yuin/goldmark/ast"
	goldrender "github.com/yuin/goldmark/renderer"
)

type renderer struct {
	source []byte
}

func New(options ...goldrender.Option) goldrender.Renderer {
	r := &renderer{}

	return r
}

func (r *renderer) Render(w io.Writer, source []byte, n ast.Node) error {
	segments := []map[string]interface{}{}

	r.source = source
	lastCodeBlock := &n

	err := ast.Walk(n, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		s := ast.WalkStatus(ast.WalkContinue)

		if n.Kind() != ast.KindFencedCodeBlock || !entering {
			return s, nil
		}

		var content strings.Builder

		switch n.Type() {
		case ast.TypeInline:
			content.Write(n.Text(source))
		default:
			for i := 0; i < n.Lines().Len(); i++ {
				line := n.Lines().At(i)
				_, _ = content.Write(source[line.Start:line.Stop])
			}
		}

		codeBlock := n.(*ast.FencedCodeBlock)
		start := r.getCodeBlockStart(codeBlock)

		if lastCodeBlock != nil {
			lastStop := r.getStop(lastCodeBlock)
			mdStr := string(source[lastStop:start])
			seg := map[string]interface{}{"Markdown": mdStr}
			segments = append(segments, seg)
			lastCodeBlock = &n
		}

		seg := map[string]interface{}{}
		seg["Code"] = content.String()
		seg["Language"] = string(codeBlock.Language(source))
		seg["Description"] = string(n.PreviousSibling().Text(source))
		segments = append(segments, seg)

		return s, nil
	})
	if err != nil {
		return err
	}

	doc := map[string]interface{}{"Document": segments}
	jStr, _ := json.Marshal(doc)
	fmt.Println(string(jStr))

	return nil
}

func (r *renderer) getCodeBlockStart(n *ast.FencedCodeBlock) int {
	if n.Info != nil {
		return n.Info.Segment.Start
	}
	return n.Lines().At(0).Start
}

func (r *renderer) getStop(n *ast.Node) int {
	l := (*n).Lines().Len()
	if l > 0 {
		return (*n).Lines().At(l - 1).Stop
	}
	return 0
}

// AddOptions has no effect
func (r *renderer) AddOptions(_ ...goldrender.Option) {
	// Nothing to do here
}

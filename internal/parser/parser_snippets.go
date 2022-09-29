package parser

import (
	"fmt"
	"strings"

	"github.com/stateful/rdme/internal/snippets"
	"github.com/yuin/goldmark/ast"
)

func (p *Parser) Snippets() (result snippets.Snippets) {
	nameIndexes := make(map[string]int)

	for c := p.rootNode.FirstChild(); c != nil; c = c.NextSibling() {
		if c.Kind() != ast.KindFencedCodeBlock {
			continue
		}

		var content strings.Builder
		for i := 0; i < c.Lines().Len(); i++ {
			line := c.Lines().At(i)
			_, _ = content.Write(p.src[line.Start:line.Stop])
		}

		strContent := content.String()
		if snippets.InvalidCommand(strContent) {
			continue
		}

		codeBlock := c.(*ast.FencedCodeBlock)

		s := snippets.Snippet{
			Attributes:  snippets.ParseAttributes(snippets.ExtractRawAttributes(p.src, codeBlock)),
			Content:     strContent,
			Description: string(c.PreviousSibling().Text(p.src)),
			Language:    string(codeBlock.Language(p.src)),
		}

		// Set a name for the snippet.
		// First option is that the name is set explicitly.
		// Other option is to get the name from the first line
		// of the snippet.
		// Both options require us to dedup names.
		var name string
		if n, ok := s.Attributes["name"]; ok && n != "" {
			name = n
		} else {
			name = snippets.SanitizeName(s.FirstLine())
		}
		nameIndexes[name]++
		if nameIndexes[name] > 1 {
			name = fmt.Sprintf("%s_%d", name, nameIndexes[name])
		}

		s.Name = name

		result = append(result, &s)
	}

	return result
}

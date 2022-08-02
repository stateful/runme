package parser

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/yuin/goldmark/ast"
)

func (p *Parser) Snippets() (result Snippets) {
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
		if invalidCommand(strContent) {
			continue
		}

		codeBlock := c.(*ast.FencedCodeBlock)

		s := Snippet{
			attributes:  parseAttributes(extractRawAttributes(p.src, codeBlock)),
			content:     strContent,
			description: string(c.PreviousSibling().Text(p.src)),
			language:    string(codeBlock.Language(p.src)),
		}

		// Set a name for the snippet.
		// First option is that the name is set explicitly.
		// Other option is to get the name from the first line
		// of the snippet.
		// Both options require us to dedup names.
		var name string
		if n, ok := s.attributes["name"]; ok && n != "" {
			name = n
		} else {
			name = sanitizeName(s.FirstLine())
		}
		nameIndexes[name]++
		if nameIndexes[name] > 1 {
			name = fmt.Sprintf("%s_%d", name, nameIndexes[name])
		}

		s.name = name

		result = append(result, &s)
	}

	return result
}

var notAllowedChars = []rune{'├', '─', '│'}

func invalidCommand(src string) bool {
	for _, char := range src {
		for _, invalid := range notAllowedChars {
			if invalid == char {
				return true
			}
		}
	}
	return false
}

func parseAttributes(raw []byte) map[string]string {
	items := bytes.Split(raw, []byte{' '})
	if len(items) == 0 {
		return nil
	}

	result := make(map[string]string)
	for _, item := range items {
		if !bytes.Contains(item, []byte{'='}) {
			continue
		}
		kv := bytes.Split(item, []byte{'='})
		result[string(kv[0])] = string(kv[1])
	}

	return result
}

func extractRawAttributes(source []byte, n *ast.FencedCodeBlock) []byte {
	if n.Info == nil {
		return nil
	}

	segment := n.Info.Segment
	info := segment.Value(source)
	start, stop := -1, -1
	for i := 0; i < len(info); i++ {
		if start == -1 && info[i] == '{' && i+1 < len(info) && info[i+1] != '}' {
			start = i + 1
		}
		if stop == -1 && info[i] == '}' {
			stop = i
			break
		}
	}

	if start >= 0 && stop >= 0 {
		return bytes.TrimSpace(info[start:stop])
	}

	return nil
}

func sanitizeName(s string) string {
	// Handle cases when the first line is defining a variable.
	if idx := strings.Index(s, "="); idx > 0 {
		return sanitizeName(s[:idx])
	}

	fragments := strings.Split(s, " ")
	if len(fragments) > 1 {
		s = strings.Join(fragments[:2], " ")
	} else {
		s = fragments[0]
	}

	var b bytes.Buffer

	for _, r := range strings.ToLower(s) {
		if r == ' ' {
			_, _ = b.WriteRune('-')
		} else if r >= '0' && r <= '9' || r >= 'a' && r <= 'z' {
			_, _ = b.WriteRune(r)
		}
	}

	return b.String()
}

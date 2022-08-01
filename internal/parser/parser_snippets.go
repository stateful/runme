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

		if s.Name() == "" {
			fragments := strings.Split(s.FirstLine(), " ")
			executable := fragments[0]

			// Usually the command looks like this: EXECUTABLE subcommand arg1, arg2, ...
			if len(fragments) > 1 {
				suffixBuf := bytes.NewBuffer(nil)
				for _, r := range strings.ToLower(fragments[1]) {
					if r >= '0' && r <= '9' || r >= 'a' && r <= 'z' {
						_, _ = suffixBuf.WriteRune(r)
					}
				}

				executable += "-" + suffixBuf.String()
			}
			nameIndexes[executable]++

			if nameIndexes[executable] == 1 {
				s.attributes["name"] = executable
			} else {
				s.attributes["name"] = fmt.Sprintf("%s_%d", executable, nameIndexes[executable])
			}
		}

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

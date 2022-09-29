package utils

import (
	"bytes"
	"strings"

	"github.com/yuin/goldmark/ast"
)

var notAllowedChars = []rune{'├', '─', '│'}

func InvalidCommand(src string) bool {
	for _, char := range src {
		for _, invalid := range notAllowedChars {
			if invalid == char {
				return true
			}
		}
	}
	return false
}

func ParseAttributes(raw []byte) map[string]string {
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

func ExtractRawAttributes(source []byte, n *ast.FencedCodeBlock) []byte {
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

func SanitizeName(s string) string {
	// Handle cases when the first line is defining a variable.
	if idx := strings.Index(s, "="); idx > 0 {
		return SanitizeName(s[:idx])
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

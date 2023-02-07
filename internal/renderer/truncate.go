package renderer

import (
	"strings"

	"github.com/cli/cli/pkg/text"
)

func TruncateClean(maxWidth int, s string) string {
	const suffix = ", ..."

	if len(s) <= maxWidth {
		return s
	}

	short := text.Truncate(maxWidth-len(suffix), s)

	if i := strings.LastIndex(short, ","); i >= 0 {
		short = short[:i] + suffix
	}

	return short
}

func TruncateMiddle(maxWidth int, t string) string {
	if len(t) <= maxWidth {
		return t
	}

	const ellipsis = "..."

	r := []rune(t)

	if maxWidth < len(ellipsis)+2 {
		return string(r[0:maxWidth])
	}

	halfWidth := (maxWidth - len(ellipsis)) / 2
	remainder := (maxWidth - len(ellipsis)) % 2

	runes := append(r[0:halfWidth+remainder], []rune(ellipsis)...)
	runes = append(runes, r[len(t)-halfWidth:]...)
	return string(runes)
}

func TruncateLeft(s string, chars string) string {
	if i := strings.IndexAny(s, chars); i >= 0 {
		s = s[i+1:]
	}
	return s
}

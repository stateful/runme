package shell

import (
	"strings"
	"unicode"
)

func StripComments(lines []string) (ret []string) {
	for _, l := range lines {
		l = strings.TrimSpace(l)

		split := strings.SplitN(l, "#", 2)

		if len(split) == 0 || split[0] == "" {
			continue
		}

		ret = append(ret, strings.TrimRightFunc(split[0], unicode.IsSpace))
	}

	return
}

func TryGetNonCommentLine(lines []string) string {
	stripped := StripComments(lines)

	if len(stripped) > 0 {
		return stripped[0]
	}

	if len(lines) > 0 {
		return lines[0]
	}

	return ""
}

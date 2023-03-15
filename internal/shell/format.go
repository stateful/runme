package shell

import "strings"

func StripComments(lines []string) (ret []string) {
	for _, l := range lines {
		l = strings.TrimSpace(l)

		if !strings.HasPrefix(l, "#") {
			ret = append(ret, l)
		}
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

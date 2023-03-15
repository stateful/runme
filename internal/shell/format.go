package shell

import "strings"

func TryGetNonCommentLine(lines []string) string {
	var line string

	for _, l := range lines {
		if l == "" {
			continue
		}

		l = strings.TrimSpace(l)

		if strings.HasPrefix(l, "#") {
			continue
		}

		line = l
		break
	}

	if line == "" && len(lines) > 0 {
		line = lines[0]
	}

	return line
}

package parser

import (
	"regexp"
	"strings"
)

type Snippet struct {
	attributes  map[string]string
	content     string
	description string // preceeding paragraph
	language    string
}

func (s Snippet) Executable() string {
	if s.language != "" {
		return s.language
	}
	return "sh"
}

func (s Snippet) Content() string {
	return strings.TrimSpace(s.content)
}

func (s Snippet) Lines() []string {
	var cmds []string

	firstHasDollar := false
	lines := strings.Split(s.content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "$") {
			firstHasDollar = true
			line = strings.TrimLeft(line, "$")
		} else if firstHasDollar {
			// If the first line was prefixed with "$",
			// then all commands should be as well.
			// If they are not, it's likely that
			// they indicate the expected output instead.
			continue
		}

		line = strings.TrimSpace(line)

		if line == "" {
			continue
		}

		cmds = append(cmds, line)
	}

	return cmds
}

func (s Snippet) FirstLine() string {
	cmds := s.Lines()
	if len(cmds) > 0 {
		return cmds[0]
	}
	return ""
}

var descriptionEndingsRe = regexp.MustCompile(`[:?!]$`)

func (s Snippet) Description() string {
	result := descriptionEndingsRe.ReplaceAllString(s.description, ".")
	return result
}

func (s Snippet) Name() string {
	return s.attributes["name"]
}

type Snippets []Snippet

func (s Snippets) Lookup(name string) (Snippet, bool) {
	for _, snippet := range s {
		if snippet.Name() == name {
			return snippet, true
		}
	}
	return Snippet{}, false
}

func (s Snippets) Names() (result []string) {
	for _, snippet := range s {
		result = append(result, snippet.Name())
	}
	return result
}

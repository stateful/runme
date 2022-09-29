package snippets

import (
	"regexp"
	"strings"
)

type Snippet struct {
	Attributes  map[string]string `json:"attributes,omitempty"`
	Content     string            `json:"content,omitempty"`
	Description string            `json:"description,omitempty"` // preceeding paragraph
	Name        string            `json:"name,omitempty"`
	Markdown    string            `json:"markdown,omitempty"`
	Language    string            `json:"language,omitempty"`
	Lines       []string          `json:"lines,omitempty"` // Kinda ugly
}

func (s *Snippet) Executable() string {
	if s.Language != "" {
		return s.Language
	}
	return "sh"
}

func (s *Snippet) GetContent() string {
	return strings.TrimSpace(s.Content)
}

func (s *Snippet) ReplaceContent(val string) {
	s.Content = val
}

func (s *Snippet) GetLines() []string {
	var cmds []string

	firstHasDollar := false
	lines := strings.Split(s.Content, "\n")

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

func (s *Snippet) FirstLine() string {
	cmds := s.GetLines()
	if len(cmds) > 0 {
		return cmds[0]
	}
	return ""
}

var descriptionEndingsRe = regexp.MustCompile(`[:?!]$`)

func (s *Snippet) GetDescription() string {
	result := descriptionEndingsRe.ReplaceAllString(s.Description, ".")
	return result
}

func (s *Snippet) GetName() string {
	return s.Name
}

type Snippets []*Snippet

func (s Snippets) Lookup(name string) (*Snippet, bool) {
	for _, snippet := range s {
		if snippet.GetName() == name {
			return snippet, true
		}
	}
	return nil, false
}

func (s Snippets) GetNames() (result []string) {
	for _, snippet := range s {
		result = append(result, snippet.GetName())
	}
	return result
}

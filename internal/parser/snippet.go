package parser

import (
	"errors"
	"regexp"
	"strings"
)

type Snippet struct {
	attributes  map[string]string
	content     string
	description string //  preceeding  paragraph
	name        string
	language    string
	parameters  []string
}

var descriptionEndingsRe = regexp.MustCompile(`[:?!]$`)
var parameterRe = regexp.MustCompile("<[a-zA-Z0-9_ ]*>")

func (s *Snippet) Executable() string {
	if s.language != "" {
		return s.language
	}
	return "sh"
}

func (s *Snippet) Content() string {
	return strings.TrimSpace(s.content)
}

func (s *Snippet) ReplaceContent(val string) {
	s.content = val
}

func (s *Snippet) Lines() []string {
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

func (s *Snippet) FirstLine() string {
	cmds := s.Lines()
	if len(cmds) > 0 {
		return cmds[0]
	}
	return ""
}

func (s *Snippet) Description() string {
	result := descriptionEndingsRe.ReplaceAllString(s.description, ".")
	return result
}

func (s *Snippet) Name() string {
	return s.name
}

type Snippets []*Snippet

func (s Snippets) Lookup(name string) (*Snippet, bool) {
	for _, snippet := range s {
		if snippet.Name() == name {
			return snippet, true
		}
	}
	return nil, false
}

func (s Snippets) Names() (result []string) {
	for _, snippet := range s {
		result = append(result, snippet.Name())
	}
	return result
}

func (s *Snippet) extractParameters() []string {
	match := parameterRe.FindAllStringSubmatch(s.content, -1)
	var parameters []string
	for _, m := range match {
		parameters = append(parameters, m[0])
	}
	return parameters
}

func (s *Snippet) mapParameterValues(parameters []string, values []string) string {
	var paramExp regexp.Regexp
	newCmd := s.Content()
	for i := range parameters {
		paramExp = *regexp.MustCompile(parameters[i])
		newCmd = paramExp.ReplaceAllString(newCmd, values[i])
	}
	return newCmd
}

func (s *Snippet) FillInParameters(values []string) error {
	parameters := s.extractParameters()
	if len(parameters) != len(values) {
		return errors.New("Mismatch between the number of parameters and the number of values provided")
	}
	s.ReplaceContent(s.mapParameterValues(parameters, values))
	return nil
}


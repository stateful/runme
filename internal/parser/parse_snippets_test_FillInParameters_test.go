package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParser_FillInParameters_one_substitution(t *testing.T) {
	// Given
	cmd := New([]byte(`
Snippet without proper annotations:
` + "```sh { name=run }" + `
go run <FILENAME>
` + "```"))

	// When
	snippets := cmd.Snippets()
	assert.Len(t, snippets, 1)
	err := snippets[0].FillInParameters([]string{"main.go"})

	// Then
	assert.Equal(t, nil, err)
	assert.EqualValues(t, []string{"run"}, snippets.Names())
	assert.EqualValues(t, "go run main.go", snippets[0].Content())
}

func TestParser_FillInParameters_mutliple_substitution(t *testing.T) {
	// Given
	cmd := New([]byte(`
Snippet without proper annotations:
` + "```sh { name=auth }" + `
gh auth login --hostname <hostname> --scopes <scope>
` + "```"))

	// When
	snippets := cmd.Snippets()
	assert.Len(t, snippets, 1)
	err := snippets[0].FillInParameters([]string{"hostname", "scope"})

	// Then
	assert.Equal(t, nil, err)
	assert.EqualValues(t, []string{"auth"}, snippets.Names())
	assert.EqualValues(t, "gh auth login --hostname hostname --scopes scope", snippets[0].Content())
}

func TestParser_FillInParameters_multiline_snippet(t *testing.T) {
	// Given
	cmd := New([]byte(`
Snippet without proper annotations:
` + "```sh { name=auth }" + `
gh auth login \
  --hostname <hostname> \
  --scopes <scope>
` + "```"))

	// When
	snippets := cmd.Snippets()
	assert.Len(t, snippets, 1)
	err := snippets[0].FillInParameters([]string{"hostname", "scope"})

	// Then
	assert.Equal(t, nil, err)
	assert.EqualValues(t, []string{"auth"}, snippets.Names())
	assert.EqualValues(t, `gh auth login \
  --hostname hostname \
  --scopes scope`, snippets[0].Content())
}

func TestParser_FillInParameters_multi_command_without_parameters(t *testing.T) {
	// Given
	cmd := New([]byte(`
Snippet without proper annotations:
` + "```sh { name=comment } " + `
gh comment <number> 
` + "```"))

	// When
	snippets := cmd.Snippets()
	assert.Len(t, snippets, 1)

	// Then
	assert.EqualValues(t, []string{"comment"}, snippets.Names())
	assert.EqualValues(t, "gh comment <number>", snippets[0].Content())
}

func TestParser_FillInParameters_missing_parameter(t *testing.T) {
	// Given
	cmd := New([]byte(`
Snippet without proper annotations:
` + "```sh { name=comment } " + `
gh comment <number> --body <message>
` + "```"))

	// When
	snippets := cmd.Snippets()
	assert.Len(t, snippets, 1)
	err := snippets[0].FillInParameters([]string{"12"})

	// Then
	assert.NotEqual(t, nil, err)
}

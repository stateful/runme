package document

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSections_WithMalformedFrontmatter(t *testing.T) {
	data := []byte(`--
a: b
---

# Heading
`)
	sections, err := ParseSections(data)
	require.NoError(t, err)

	assert.Equal(t, string(data), string(sections.Content))
	assert.Equal(t, "", string(sections.Frontmatter))
	assert.Equal(t, 0, sections.ContentOffset)
}

func TestParseSections_WithoutFrontmatter(t *testing.T) {
	data := []byte(`# Example

A paragraph
`)
	sections, err := ParseSections(data)
	require.NoError(t, err)
	assert.Equal(t, string(data), string(sections.Content))
}

func TestParseSections_WithFrontmatterYAML(t *testing.T) {
	data := []byte(`---
prop1: val1
prop2: val2
---

# Example

A paragraph
`)
	sections, err := ParseSections(data)
	require.NoError(t, err)
	assert.Equal(t, "---\nprop1: val1\nprop2: val2\n---", string(sections.Frontmatter))
	assert.Equal(t, "# Example\n\nA paragraph\n", string(sections.Content))
}

func TestParseSections_WithFrontmatterTOML(t *testing.T) {
	data := []byte(`+++
prop1 = "val1"
prop2 = "val2"
+++

# Example

A paragraph
`)
	sections, err := ParseSections(data)
	require.NoError(t, err)
	assert.Equal(t, "+++\nprop1 = \"val1\"\nprop2 = \"val2\"\n+++", string(sections.Frontmatter))
	assert.Equal(t, "# Example\n\nA paragraph\n", string(sections.Content))
}

func TestParseSections_WithFrontmatterJSON(t *testing.T) {
	data := []byte(`{
    "prop1": "val1",
    "prop2": "val2"
}

# Example

A paragraph
`)
	sections, err := ParseSections(data)
	require.NoError(t, err)
	assert.Equal(t, "{\n    \"prop1\": \"val1\",\n    \"prop2\": \"val2\"\n}", string(sections.Frontmatter))
	assert.Equal(t, "# Example\n\nA paragraph\n", string(sections.Content))
}

func TestParseSections_SkipIncludes(t *testing.T) {
	data := []byte(`{% include "gradual-rollout-intro.md" %}

## Procedure

You can configure the rollout-duration parameter by modifying the config-network ConfigMap, or by using the Operator.
`)
	sections, err := ParseSections(data)
	require.NoError(t, err)
	assert.Equal(t, "", string(sections.Frontmatter))
	assert.Equal(t, "{% include \"gradual-rollout-intro.md\" %}\n\n## Procedure\n\nYou can configure the rollout-duration parameter by modifying the config-network ConfigMap, or by using the Operator.\n", string(sections.Content))
}

func TestParseSections_SkipTemplates(t *testing.T) {
	data := []byte(`{{- $repourl := $.Info.RepositoryURL -}}
# CHANGELOG
All notable changes to this project will be documented in this file.
`)
	sections, err := ParseSections(data)
	require.NoError(t, err)
	assert.Equal(t, "", string(sections.Frontmatter))
	assert.Equal(t, "{{- $repourl := $.Info.RepositoryURL -}}\n# CHANGELOG\nAll notable changes to this project will be documented in this file.\n", string(sections.Content))
}

func TestParseSections_SkipMkdocsScissorSyntax(t *testing.T) {
	data := []byte("--8<-- \"snippets/live-code-snippets-button-executor.md\"\n\n## Import Dynatrace Dashboard\n\n``` {\"name\": \"docker ps\"}\ndocker ps\n```\n")
	sections, err := ParseSections(data)
	require.NoError(t, err)
	assert.Equal(t, "", string(sections.Frontmatter))
	assert.NotEqual(t, "{{- $repourl := $.Info.RepositoryURL -}}\n# CHANGELOG\nAll notable changes to this project will be documented in this file.\n", string(sections.Content))
}

func TestParser(t *testing.T) {
	t.Run("VaryingWidthCharacters", func(t *testing.T) {
		cyrillicH1 := []byte(`# Заголовок`)

		l := &itemParser{input: cyrillicH1}
		runItemParser(l, parseInit)

		for _, item := range l.items {
			switch item.Type() {
			case parsedItemContent:
				actual := item.Value(cyrillicH1)
				require.Equal(t, cyrillicH1, actual)
			default:
				t.Errorf("unexpected item type: %v", item.Type())
			}
		}
	})

	t.Run("ShortContent", func(t *testing.T) {
		singleCharMultiByte := []byte(`😆`)

		l := &itemParser{input: singleCharMultiByte}
		runItemParser(l, parseInit)

		for _, item := range l.items {
			switch item.Type() {
			case parsedItemContent:
				actual := item.Value(singleCharMultiByte)
				require.Equal(t, singleCharMultiByte, actual)
			default:
				t.Errorf("unexpected item type: %v", item.Type())
			}
		}
	})
}

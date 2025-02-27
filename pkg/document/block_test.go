package document

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBlock_Tags(t *testing.T) {
	t.Run("SupersetIncludingLegacyCategories", func(t *testing.T) {
		block := &CodeBlock{
			attributes: NewAttributesWithFormat(
				map[string]string{
					"category": "cat1,cat2",
					"tag":      "tag1,tag2",
				},
				"json",
			),
		}
		assert.Equal(t, []string{"cat1", "cat2", "tag1", "tag2"}, block.Tags())
	})
}

func TestBlock_Ignored(t *testing.T) {
	t.Run("DefaultTrue", func(t *testing.T) {
		block := &CodeBlock{}
		assert.False(t, block.Ignored())
	})

	t.Run("TransformAttributeFalse", func(t *testing.T) {
		block := &CodeBlock{
			attributes: NewAttributesWithFormat(
				map[string]string{
					"transform": "false",
				},
				"json",
			),
		}
		assert.True(t, block.Ignored())
	})

	t.Run("IgnoreAttributeTrue", func(t *testing.T) {
		block := &CodeBlock{
			attributes: NewAttributesWithFormat(
				map[string]string{
					"ignore": "true",
				},
				"json",
			),
		}
		assert.True(t, block.Ignored())
	})
}

func TestBlock_Mermaid(t *testing.T) {
	t.Run("IgnoreByDefault", func(t *testing.T) {
		block := &CodeBlock{
			attributes: NewAttributesWithFormat(
				map[string]string{},
				"json",
			),
			language: "mermaid",
		}
		assert.True(t, block.Ignored())
	})

	t.Run("IncludeWithTransform", func(t *testing.T) {
		block := &CodeBlock{
			attributes: NewAttributesWithFormat(
				map[string]string{
					"transform": "true",
				},
				"json",
			),
			language: "mermaid",
		}
		assert.False(t, block.Ignored())
	})

	t.Run("IncludeWithNegatedIgnore", func(t *testing.T) {
		block := &CodeBlock{
			attributes: NewAttributesWithFormat(
				map[string]string{
					"ignore": "false",
				},
				"json",
			),
			language: "mermaid",
		}
		assert.False(t, block.Ignored())
	})
}

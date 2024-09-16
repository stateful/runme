package document

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
)

func TestBlock_Clone(t *testing.T) {
	block := &CodeBlock{
		id:            "id",
		idGenerated:   false,
		attributes:    map[string]string{"key": "value"},
		document:      nil,
		inner:         nil,
		intro:         "intro",
		language:      "language",
		lines:         []string{"line1", "line2"},
		name:          "name",
		nameGenerated: false,
		value:         []byte("value"),
	}
	clone := block.Clone()
	assert.True(t, cmp.Equal(block, clone, cmp.AllowUnexported(CodeBlock{})), "expected %v, got %v", block, clone)

	block.attributes["key"] = "new-value"
	assert.NotEqual(t, block.attributes["key"], clone.attributes["key"])

	block.lines[0] = "new-line"
	assert.NotEqual(t, block.lines[0], clone.lines[0])

	block.value[0] = 'a'
	assert.NotEqual(t, block.value[0], clone.value[0])
}

func TestBlock_Tags(t *testing.T) {
	t.Run("Superset including legacy categories", func(t *testing.T) {
		block := &CodeBlock{
			attributes: map[string]string{
				"category": "cat1,cat2",
				"tag":      "tag1,tag2",
			},
		}
		assert.Equal(t, []string{"cat1", "cat2", "tag1", "tag2"}, block.Tags())
	})
}

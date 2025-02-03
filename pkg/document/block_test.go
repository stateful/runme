package document

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBlock_Tags(t *testing.T) {
	t.Run("Superset including legacy categories", func(t *testing.T) {
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

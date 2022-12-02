package document

import (
	"testing"

	"github.com/stateful/runme/internal/renderer/cmark"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDocument_Parse(t *testing.T) {
	data := []byte(`# Examples

First paragraph.

> bq1
>
>     echo "inside bq"
>
> bq2
> bq3

1. Item 1

   ` + "```" + `sh {name=echo first= second=2}
   $ echo "Hello, runme!"
   ` + "```" + `

   Inner paragraph

2. Item 2
3. Item 3
`)
	doc := New(data, cmark.Render)
	node, _, err := doc.Parse()
	require.NoError(t, err)

	assert.Len(t, node.children, 4)
	assert.Len(t, node.children[2].children, 3)
	assert.Len(t, node.children[3].children, 3)
	assert.Len(t, node.children[3].children[0].children, 3)

	assert.Equal(t, "> bq1\n>\n>     echo \"inside bq\"\n>\n> bq2\n> bq3\n", string(node.children[2].Item().Value()))
	assert.Equal(t, "1. Item 1\n\n   ```sh {name=echo first= second=2}\n   $ echo \"Hello, runme!\"\n   ```\n\n   Inner paragraph\n", string(node.children[3].children[0].Item().Value()))
	assert.Equal(t, "2. Item 2\n", string(node.children[3].children[1].Item().Value()))
	assert.Equal(t, "3. Item 3\n", string(node.children[3].children[2].Item().Value()))
}

package document

import (
	"testing"

	"github.com/stateful/runme/internal/renderer/md"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yuin/goldmark/ast"
)

func TestDocument_BlocksTree(t *testing.T) {
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
	doc := NewDocument(data, md.Render)
	node, err := doc.Parse()
	require.NoError(t, err)
	assert.Len(t, node.children, 4)
	assert.Len(t, node.children[2].children, 3)
	assert.Len(t, node.children[3].children, 3)
	assert.Len(t, node.children[3].children[0].children, 3)

	assert.Equal(t, "> bq1\n>\n>     echo \"inside bq\"\n>\n> bq2\n> bq3\n", string(node.children[2].Item().Value()))
	assert.Equal(t, "1. Item 1\n\n   ```sh {name=echo first= second=2}\n   $ echo \"Hello, runme!\"\n   ```\n\n   Inner paragraph\n", string(node.children[3].children[0].Item().Value()))
	assert.Equal(t, "2. Item 2\n", string(node.children[3].children[1].Item().Value()))
	assert.Equal(t, "3. Item 3\n", string(node.children[3].children[2].Item().Value()))

	// Validate if rendering using the blocks tree works as expected.
	t.Run("RenderWithSourceProvider", func(t *testing.T) {
		result, err := md.RenderWithSourceProvider(
			doc.parse(),
			data,
			func(astNode ast.Node) ([]byte, bool) {
				result := FindByInner(node, astNode)
				if result != nil {
					return result.Item().Value(), true
				}
				return nil, false
			},
		)
		require.NoError(t, err)
		assert.Equal(t, string(data), string(result))
	})
}

func TestDocument_CodeBlocks(t *testing.T) {
	data := []byte(`> bq1
>
>     echo "inside bq but not fenced"
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

` + "```sh" + `
echo 1
` + "```" + `
`)
	doc := NewDocument(data, md.Render)
	node, err := doc.Parse()
	require.NoError(t, err)
	codeBlocks := CollectCodeBlocks(node)
	assert.Len(t, codeBlocks, 2)
	assert.Equal(t, "```sh {name=echo first= second=2}\n$ echo \"Hello, runme!\"\n```\n", string(codeBlocks[0].Value()))
	assert.Equal(t, "```sh\necho 1\n```\n", string(codeBlocks[1].Value()))
}

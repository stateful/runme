package renderer

import (
	"testing"

	"github.com/stateful/runme/internal/document"
	"github.com/stateful/runme/internal/renderer/md"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yuin/goldmark/ast"
)

func TestRenderWithSourceProvider(t *testing.T) {
	data := []byte(`# Examples

It can have an annotation with a name:

` + "```" + `sh {name=echo first= second=2}
$ echo "Hello, runme!"
` + "```" + `

> bq 1
> bq 2
>
>     echo 1
>
> b1 3

1. Item 1

   ` + "```" + `sh {name=echo first= second=2}
   $ echo "Hello, runme!"
   ` + "```" + `

   Inner paragraph

2. Item 2
`)
	source := document.NewSource(data, md.Render)
	parsed := source.Parse()
	tree, err := parsed.BlocksTree()
	require.NoError(t, err)
	result, err := md.RenderWithSourceProvider(
		parsed.Root(),
		data,
		func(astNode ast.Node) ([]byte, bool) {
			result := document.FindByInner(tree, astNode)
			if result != nil {
				return result.Item().Value(), true
			}
			return nil, false
		},
	)
	require.NoError(t, err)
	assert.Equal(t, string(data), string(result))
}

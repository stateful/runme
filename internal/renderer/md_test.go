package renderer

import (
	"testing"

	"github.com/stateful/runme/internal/document"
	"github.com/stateful/runme/internal/renderer/md"
	"github.com/stretchr/testify/require"
	"github.com/yuin/goldmark/ast"
)

type sourceProvider struct {
	node *document.Node
}

func (p *sourceProvider) Value(astNode ast.Node) ([]byte, bool) {
	result := document.FindByInner(p.node, astNode)
	if result != nil {
		return result.Value().Value(), true
	}
	return nil, false
}

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
	source := document.NewSource(data)
	parsed := source.Parse()
	tree := parsed.BlocksTree()
	result, err := md.RenderWithSourceProvider(parsed.Root(), data, &sourceProvider{node: tree})
	require.NoError(t, err)
	require.Equal(t, string(data), string(result))
}

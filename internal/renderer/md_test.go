package renderer

import (
	"testing"

	"github.com/stateful/runme/internal/document"
	"github.com/stateful/runme/internal/renderer/md"
	"github.com/stretchr/testify/require"
	"github.com/yuin/goldmark/ast"
)

type sourceProvider struct {
	blocks document.Blocks
}

func (p *sourceProvider) Value(node ast.Node) ([]byte, bool) {
	for _, block := range p.blocks {
		if node == block.Unwrap() {
			return block.Value(), true
		}
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
> b1 3

1. Item 1

   ` + "```" + `sh {name=echo first= second=2}
   $ echo "Hello, runme!"
   ` + "```" + `

2. Item 2
`)
	source := document.NewSource(data)
	parsed := source.Parse()
	blocks := parsed.Blocks()

	result, err := md.RenderWithSourceProvider(parsed.Root(), data, &sourceProvider{blocks: blocks})
	require.NoError(t, err)
	require.Equal(t, string(data), string(result))
}

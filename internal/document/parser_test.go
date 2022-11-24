package document

import (
	"testing"

	"github.com/stateful/runme/internal/renderer/md"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsedSource_BlocksTree(t *testing.T) {
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
	source := NewSource(data, md.Render)
	parsed := source.Parse()
	tree, err := parsed.BlocksTree()
	require.NoError(t, err)
	assert.Len(t, tree.children, 4)
	assert.Len(t, tree.children[2].children, 3)
	assert.Len(t, tree.children[3].children, 3)
	assert.Len(t, tree.children[3].children[0].children, 3)
}

func TestParsedSource_CodeBlocks(t *testing.T) {
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
	source := NewSource(data, md.Render)
	parsed := source.Parse()
	tree, err := parsed.BlocksTree()
	require.NoError(t, err)
	var codeBlocks CodeBlocks
	CollectCodeBlocks(tree, &codeBlocks)
	assert.Len(t, codeBlocks, 2)
}

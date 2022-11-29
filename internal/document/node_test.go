package document

import (
	"testing"

	"github.com/stateful/runme/internal/renderer/md"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToCells(t *testing.T) {
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

   Before inner paragraph

   ` + "```" + `sh {name=echo first= second=2}
   $ echo "Hello, runme!"
   ` + "```" + `

   First inner paragraph

   Second inner paragraph

2. Item 2
3. Item 3
`)
	source := NewSource(data, md.Render)
	parsed := source.Parse()
	tree, err := parsed.BlocksTree()
	require.NoError(t, err)

	var cells []*Cell
	ToCells(tree, &cells, data)
	assert.Len(t, cells, 9)
	assert.Equal(t, "# Examples\n", cells[0].Value)
	assert.Equal(t, "It can have an annotation with a name:\n", cells[1].Value)
	assert.Equal(t, "```sh {name=echo first= second=2}\n$ echo \"Hello, runme!\"\n```\n", cells[2].Value)
	assert.Equal(t, "> bq 1\n> bq 2\n>     \n>     echo 1\n>\n> b1 3\n", cells[3].Value)
	assert.Equal(t, "1. Item 1", cells[4].Value)
	assert.Equal(t, "```sh {name=echo first= second=2}\n$ echo \"Hello, runme!\"\n```\n", cells[5].Value)
	assert.Equal(t, "First inner paragraph", cells[6].Value)
	assert.Equal(t, "Second inner paragraph", cells[7].Value)
	assert.Equal(t, "2. Item 2\n\n3. Item 3\n", cells[8].Value)
}

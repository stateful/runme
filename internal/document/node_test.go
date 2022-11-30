package document

import (
	"testing"

	"github.com/stateful/runme/internal/renderer/md"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToCells_Basic(t *testing.T) {
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

   First inner paragraph

   Second inner paragraph

2. Item 2
3. Item 3
`)
	doc := NewDocument(data, md.Render)
	node, err := doc.Parse()
	require.NoError(t, err)

	cells := ToCells(node, data)
	assert.Len(t, cells, 10)
	assert.Equal(t, "# Examples\n", cells[0].Value)
	assert.Equal(t, "It can have an annotation with a name:\n", cells[1].Value)
	assert.Equal(t, "```sh {name=echo first= second=2}\n$ echo \"Hello, runme!\"\n```\n", cells[2].Value)
	assert.Equal(t, "> bq 1\n> bq 2\n>\n>     echo 1\n>\n> b1 3\n", cells[3].Value)
	// TODO: fix this to contain the prefix
	assert.Equal(t, "Item 1\n", cells[4].Value)
	assert.Equal(t, "```sh {name=echo first= second=2}\n$ echo \"Hello, runme!\"\n```\n", cells[5].Value)
	assert.Equal(t, "First inner paragraph\n", cells[6].Value)
	assert.Equal(t, "Second inner paragraph\n", cells[7].Value)
	assert.Equal(t, "2. Item 2\n", cells[8].Value)
	assert.Equal(t, "3. Item 3\n", cells[9].Value)
}

func TestToCells_NoCodeBlock(t *testing.T) {
	data := []byte(`# Examples

1. Item 1
2. Item 2
3. Item 3
`)
	doc := NewDocument(data, md.Render)
	node, err := doc.Parse()
	require.NoError(t, err)

	cells := ToCells(node, data)
	assert.Len(t, cells, 2)
	assert.Equal(t, "# Examples\n", cells[0].Value)
	assert.Equal(t, "1. Item 1\n2. Item 2\n3. Item 3\n", cells[1].Value)
}

func TestApplyCells(t *testing.T) {
	data := []byte(`# Examples

1. Item 1
2. Item 2
3. Item 3

Last paragraph.
`)
	doc := NewDocument(data, md.Render)
	node, err := doc.Parse()
	require.NoError(t, err)

	cells := ToCells(node, data)
	assert.Len(t, cells, 3)

	// simply change a value of a cell
	cells[0].Value = "# Changed value\n"
	ApplyCells(node, cells)
	assert.Equal(t, "# Changed value\n", string(node.children[0].Item().Value()))

	// add a new cell
	cells = append(cells[:2], cells[1:]...)
	cells[1] = &Cell{
		Kind:     MarkupKind,
		Value:    "A new paragraph.\n",
		Metadata: map[string]any{},
	}
	ApplyCells(node, cells)
	assert.Equal(t, "# Changed value\n", string(node.children[0].Item().Value()))
	assert.Equal(t, "A new paragraph.\n", string(node.children[1].Item().Value()))
	assert.Equal(t, "1. Item 1\n2. Item 2\n3. Item 3\n", string(node.children[2].Item().Value()))

	// delete a cell
	// TODO: implement
}

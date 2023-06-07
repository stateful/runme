package document

import (
	"bytes"
	"fmt"
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
	assert.Len(t, node.children[0].children, 0)
	assert.Len(t, node.children[1].children, 0)
	assert.Len(t, node.children[2].children, 3)
	assert.Equal(t, "> bq1\n>\n>     echo \"inside bq\"\n>\n> bq2\n> bq3\n", string(node.children[2].Item().Value()))
	assert.Len(t, node.children[2].children[0].children, 0)
	assert.Equal(t, "bq1\n", string(node.children[2].children[0].Item().Value()))
	assert.Len(t, node.children[2].children[1].children, 0)
	assert.Equal(t, "    echo \"inside bq\"\n", string(node.children[2].children[1].Item().Value()))
	assert.Len(t, node.children[2].children[2].children, 0)
	assert.Equal(t, "bq2\nbq3\n", string(node.children[2].children[2].Item().Value()))
	assert.Len(t, node.children[3].children, 3)
	assert.Len(t, node.children[3].children[0].children, 3)
	assert.Equal(t, "1. Item 1\n\n   ```sh {name=echo first= second=2}\n   $ echo \"Hello, runme!\"\n   ```\n\n   Inner paragraph\n", string(node.children[3].children[0].Item().Value()))
	assert.Len(t, node.children[3].children[0].children[0].children, 0)
	assert.Equal(t, "Item 1\n", string(node.children[3].children[0].children[0].Item().Value()))
	assert.Len(t, node.children[3].children[0].children[1].children, 0)
	assert.Equal(t, "```sh {name=echo first= second=2}\n$ echo \"Hello, runme!\"\n```\n", string(node.children[3].children[0].children[1].Item().Value()))
	assert.Len(t, node.children[3].children[0].children[2].children, 0)
	assert.Equal(t, "Inner paragraph\n", string(node.children[3].children[0].children[2].Item().Value()))
	assert.Len(t, node.children[3].children[1].children, 1)
	assert.Equal(t, "2. Item 2\n", string(node.children[3].children[1].Item().Value()))
	assert.Len(t, node.children[3].children[1].children[0].children, 0)
	assert.Equal(t, "Item 2\n", string(node.children[3].children[1].children[0].Item().Value()))
	assert.Len(t, node.children[3].children[2].children, 1)
	assert.Equal(t, "3. Item 3\n", string(node.children[3].children[2].Item().Value()))
	assert.Len(t, node.children[3].children[2].children[0].children, 0)
	assert.Equal(t, "Item 3\n", string(node.children[3].children[2].children[0].Item().Value()))
}

func TestDocument_FinalLineBreaks(t *testing.T) {
	data := []byte(`## Newline debugging

This will test final line breaks`)

	t.Run("No newline", func(t *testing.T) {
		doc := New(data, cmark.Render)
		_, astNode, err := doc.Parse()
		require.NoError(t, err)

		actual, err := cmark.Render(astNode, data)
		require.NoError(t, err)

		assert.Equal(
			t,
			string(actual),
			string(data),
		)
	})

	t.Run("7 line breaks", func(t *testing.T) {
		withLineBreaks := append(data, bytes.Repeat([]byte{'\n'}, 7)...)
		doc := New(withLineBreaks, cmark.Render)
		_, astNode, err := doc.Parse()
		require.NoError(t, err)

		actual, err := cmark.Render(astNode, withLineBreaks)
		require.NoError(t, err)

		fmt.Sprintln(string(withLineBreaks))

		assert.Equal(
			t,
			string(actual),
			string(withLineBreaks),
		)
	})
}

package edit

import (
	"testing"

	"github.com/stateful/runme/internal/document"
	"github.com/stateful/runme/internal/renderer/cmark"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testDataNested = []byte(`# Examples

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

func Test_toCells(t *testing.T) {
	t.Run("TestDataNested", func(t *testing.T) {
		doc := document.New(testDataNested, cmark.Render)
		node, _, err := doc.Parse()
		require.NoError(t, err)

		cells := toCells(node, testDataNested)
		assert.Len(t, cells, 10)
		assert.Equal(t, "# Examples\n", cells[0].Value)
		assert.Equal(t, "It can have an annotation with a name:\n", cells[1].Value)
		assert.Equal(t, "```sh {name=echo first= second=2}\n$ echo \"Hello, runme!\"\n```\n", cells[2].Value)
		assert.Equal(t, "> bq 1\n> bq 2\n>\n>     echo 1\n>\n> b1 3\n", cells[3].Value)
		assert.Equal(t, "1. Item 1\n", cells[4].Value)
		assert.Equal(t, "```sh {name=echo first= second=2}\n$ echo \"Hello, runme!\"\n```\n", cells[5].Value)
		assert.Equal(t, "First inner paragraph\n", cells[6].Value)
		assert.Equal(t, "Second inner paragraph\n", cells[7].Value)
		assert.Equal(t, "2. Item 2\n", cells[8].Value)
		assert.Equal(t, "3. Item 3\n", cells[9].Value)
	})

	t.Run("ListWithoutCode", func(t *testing.T) {
		data := []byte(`# Examples

1. Item 1
2. Item 2
3. Item 3
`)
		doc := document.New(data, cmark.Render)
		node, _, err := doc.Parse()
		require.NoError(t, err)

		cells := toCells(node, data)
		assert.Len(t, cells, 2)
		assert.Equal(t, "# Examples\n", cells[0].Value)
		assert.Equal(t, "1. Item 1\n2. Item 2\n3. Item 3\n", cells[1].Value)
	})
}

func Test_serializeCells(t *testing.T) {
	data := []byte(`# Examples

1. Item 1
2. Item 2
3. Item 3

Last paragraph.
`)

	parse := func() []*Cell {
		doc := document.New(data, cmark.Render)
		node, _, err := doc.Parse()
		require.NoError(t, err)
		cells := toCells(node, data)
		assert.Len(t, cells, 3)
		return cells
	}

	t.Run("SimpleEdit", func(t *testing.T) {
		cells := parse()
		cells[0].Value = "# New header"
		assert.Equal(
			t,
			"# New header\n\n1. Item 1\n2. Item 2\n3. Item 3\n\nLast paragraph.\n",
			string(serializeCells(cells)),
		)
	})

	t.Run("InsertListItem", func(t *testing.T) {
		cells := parse()
		cells[1].Value = "1. Item 1\n2. Item 2\n3. Item 3\n4. Item 4\n"
		assert.Equal(
			t,
			"# Examples\n\n1. Item 1\n2. Item 2\n3. Item 3\n4. Item 4\n\nLast paragraph.\n",
			string(serializeCells(cells)),
		)
	})

	t.Run("AddNewCell", func(t *testing.T) {
		t.Run("First", func(t *testing.T) {
			cells := parse()
			cells = append([]*Cell{
				{
					Kind:     MarkupKind,
					Value:    "# Title",
					Metadata: map[string]any{},
				},
			}, cells...)
			assert.Equal(
				t,
				"# Title\n\n# Examples\n\n1. Item 1\n2. Item 2\n3. Item 3\n\nLast paragraph.\n",
				string(serializeCells(cells)),
			)
		})

		t.Run("Middle", func(t *testing.T) {
			cells := parse()
			cells = append(cells[:2], cells[1:]...)
			cells[1] = &Cell{
				Kind:     MarkupKind,
				Value:    "A new paragraph.\n",
				Metadata: map[string]any{},
			}
			assert.Equal(
				t,
				"# Examples\n\nA new paragraph.\n\n1. Item 1\n2. Item 2\n3. Item 3\n\nLast paragraph.\n",
				string(serializeCells(cells)),
			)
		})

		t.Run("Last", func(t *testing.T) {
			cells := parse()
			cells = append(cells, &Cell{
				Kind:     MarkupKind,
				Value:    "Paragraph after the last one.\n",
				Metadata: map[string]any{},
			})
			assert.Equal(
				t,
				"# Examples\n\n1. Item 1\n2. Item 2\n3. Item 3\n\nLast paragraph.\n\nParagraph after the last one.\n",
				string(serializeCells(cells)),
			)
		})
	})

	t.Run("RemoveCell", func(t *testing.T) {
		cells := parse()
		cells = append(cells[:1], cells[2:]...)
		assert.Equal(
			t,
			"# Examples\n\nLast paragraph.\n",
			string(serializeCells(cells)),
		)
	})
}

func Test_serializeCells_nestedCode(t *testing.T) {
	data := []byte(`# Example

1. Item

   ` + "```sh" + `
   echo 1
   ` + "```" + `

2. Item
`)
	doc := document.New(data, cmark.Render)
	node, _, err := doc.Parse()
	require.NoError(t, err)
	cells := toCells(node, testDataNested)
	assert.Equal(
		t,
		string(data),
		string(serializeCells(cells)),
	)
}

func Test_serializeCells_addLine(t *testing.T) {
	data := []byte(`# Examples

Paragraph 1
Paragraph 2
`)

	doc := document.New(data, cmark.Render)
	node, _, err := doc.Parse()
	require.NoError(t, err)
	cells := toCells(node, data)
	assert.Len(t, cells, 2)

	cells[1].Value = "Paragraph 1\nParagraph 2\nParagraph 3\n"

	assert.Equal(
		t,
		`# Examples

Paragraph 1
Paragraph 2
Paragraph 3
`,
		string(serializeCells(cells)),
	)
}

package edit

import (
	"bytes"
	"testing"

	"github.com/stateful/runme/internal/document"
	"github.com/stateful/runme/internal/renderer/cmark"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	testDataNested = []byte(`# Examples

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

   ` + "```" + `sh {name=echo-2 first= second=2}
   $ echo "Hello, runme!"
   ` + "```" + `

   First inner paragraph

   Second inner paragraph

2. Item 2
3. Item 3
`)

	testDataNestedFlattened = []byte(`# Examples

It can have an annotation with a name:

` + "```" + `sh { name=echo first= second=2 }
$ echo "Hello, runme!"
` + "```" + `

> bq 1
> bq 2
>
>     echo 1
>
> b1 3

1. Item 1

` + "```" + `sh { name=echo-2 first= second=2 }
$ echo "Hello, runme!"
` + "```" + `

First inner paragraph

Second inner paragraph

2. Item 2

3. Item 3
`)
)

func Test_toCells_DataNested(t *testing.T) {
	doc := document.New(testDataNested, cmark.Render)
	node, _, err := doc.Parse()
	require.NoError(t, err)
	cells := toCells(node, testDataNested)
	assert.Len(t, cells, 10)
	assert.Equal(t, "# Examples", cells[0].Value)
	assert.Equal(t, "It can have an annotation with a name:", cells[1].Value)
	assert.Equal(t, "$ echo \"Hello, runme!\"", cells[2].Value)
	assert.Equal(t, "> bq 1\n> bq 2\n>\n>     echo 1\n>\n> b1 3", cells[3].Value)
	assert.Equal(t, "1. Item 1", cells[4].Value)
	assert.Equal(t, "$ echo \"Hello, runme!\"", cells[5].Value)
	assert.Equal(t, "First inner paragraph", cells[6].Value)
	assert.Equal(t, "Second inner paragraph", cells[7].Value)
	assert.Equal(t, "2. Item 2", cells[8].Value)
	assert.Equal(t, "3. Item 3", cells[9].Value)
}

func Test_toCells_Lists(t *testing.T) {
	t.Run("ListWithoutCode", func(t *testing.T) {
		data := []byte(`1. Item 1
2. Item 2
3. Item 3
`)
		doc := document.New(data, cmark.Render)
		node, _, err := doc.Parse()
		require.NoError(t, err)
		cells := toCells(node, data)
		assert.Len(t, cells, 1)
		assert.Equal(t, "1. Item 1\n2. Item 2\n3. Item 3", cells[0].Value)
	})

	t.Run("ListWithCode", func(t *testing.T) {
		data := []byte(`1. Item 1
2. Item 2
   ` + "```sh" + `
   echo 1
   ` + "```" + `
3. Item 3
`)
		doc := document.New(data, cmark.Render)
		node, _, err := doc.Parse()
		require.NoError(t, err)
		cells := toCells(node, data)
		assert.Len(t, cells, 4)
		assert.Equal(t, "1. Item 1", cells[0].Value)
		assert.Equal(t, "2. Item 2", cells[1].Value)
		assert.Equal(t, "echo 1", cells[2].Value)
		assert.Equal(t, "3. Item 3", cells[3].Value)
	})
}

func Test_toCells_EmptyLang(t *testing.T) {
	data := []byte("```" + `
echo 1
` + "```" + `
`)
	doc := document.New(data, cmark.Render)
	node, _, err := doc.Parse()
	require.NoError(t, err)
	cells := toCells(node, data)
	assert.Len(t, cells, 1)
	cell := cells[0]
	assert.Equal(t, CodeKind, cell.Kind)
	assert.Equal(t, "echo 1", cell.Value)
}

func Test_toCells_UnsupportedLang(t *testing.T) {
	data := []byte("```py { readonly=true }" + `
def hello():
    print("Hello World")
` + "```" + `
`)
	doc := document.New(data, cmark.Render)
	node, _, err := doc.Parse()
	require.NoError(t, err)
	cells := toCells(node, data)
	assert.Len(t, cells, 1)
	cell := cells[0]
	assert.Equal(t, MarkupKind, cell.Kind)
	assert.Equal(t, "```py { readonly=true }\ndef hello():\n    print(\"Hello World\")\n```", cell.Value)
}

func Test_serializeCells_Edited(t *testing.T) {
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

	t.Run("ChangeInplace", func(t *testing.T) {
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
				Value:    "Paragraph after the last one.",
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
	data := []byte(`# Development

1. Ensure you have [dev](https://github.com/stateful/dev) setup and running

2. Install MacOS dependencies

   ` + "```" + `sh
   brew bundle --no-lock
   ` + "```" + `

3. Setup pre-commit

   ` + "```" + `sh
   pre-commit install
   ` + "```" + `
`)
	doc := document.New(data, cmark.Render)
	node, _, err := doc.Parse()
	require.NoError(t, err)
	cells := toCells(node, data)
	assert.Equal(
		t,
		`# Development

1. Ensure you have [dev](https://github.com/stateful/dev) setup and running

2. Install MacOS dependencies

`+"```"+`sh
brew bundle --no-lock
`+"```"+`

3. Setup pre-commit

`+"```"+`sh
pre-commit install
`+"```"+`
`,
		string(serializeCells(cells)),
	)
}

func Test_serializeCells_attributes(t *testing.T) {
	data := []byte("```sh { name=echo first= second=2 }\necho 1\n```\n")
	doc := document.New(data, cmark.Render)
	node, _, err := doc.Parse()
	require.NoError(t, err)
	cells := toCells(node, data)
	assert.Equal(t, string(data), string(serializeCells(cells)))
}

func Test_serializeCells_privateFields(t *testing.T) {
	data := []byte("```sh { name=echo first= second=2 }\necho 1\n```\n")
	doc := document.New(data, cmark.Render)
	node, _, err := doc.Parse()
	require.NoError(t, err)

	cells := toCells(node, data)
	// Add private fields whcih will be filtered out durign serialization.
	cells[0].Metadata["_private"] = "private"
	cells[0].Metadata["runme.dev/internal"] = "internal"

	assert.Equal(t, string(data), string(serializeCells(cells)))
}

func Test_serializeCells_UnsupportedLang(t *testing.T) {
	data := []byte(`## Non-Supported Languages

` + "```py { readonly=true }" + `
def hello():
    print("Hello World")
` + "```" + `
`)
	doc := document.New(data, cmark.Render)
	node, _, err := doc.Parse()
	require.NoError(t, err)
	cells := toCells(node, data)
	assert.Equal(t, string(data), string(serializeCells(cells)))
}

func Test_serializeFencedCodeAttributes(t *testing.T) {
	t.Run("NoMetadata", func(t *testing.T) {
		var buf bytes.Buffer
		serializeFencedCodeAttributes(&buf, &Cell{
			Metadata: nil,
		})
		assert.Equal(t, "", buf.String())
	})

	t.Run("OnlyPrivateMetadata", func(t *testing.T) {
		var buf bytes.Buffer
		serializeFencedCodeAttributes(&buf, &Cell{
			Metadata: map[string]any{
				"_key":              "_value",
				"runme.dev/private": "private",
				"index":             "index",
			},
		})
		assert.Equal(t, "", buf.String())
	})

	t.Run("NamePriority", func(t *testing.T) {
		var buf bytes.Buffer
		serializeFencedCodeAttributes(&buf, &Cell{
			Metadata: map[string]any{
				"a":    "a",
				"b":    "b",
				"c":    "c",
				"name": "name",
			},
		})
		assert.Equal(t, " { name=name a=a b=b c=c }", buf.String())
	})
}

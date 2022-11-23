package document

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBlocks(t *testing.T) {
	data := []byte(`# Examples

` + "```" + `sh
$ echo "Hello, runme!"
` + "```" + `

It can have an annotation with a name:

` + "```" + `sh {name=echo first= second=2}
$ echo "Hello, runme!"
` + "```")
	source := NewSource(data)
	blocks := source.Parse().Blocks()
	assert.Equal(t, "# Examples", string(blocks[0].Value()))
	assert.Equal(t, "```sh\n$ echo \"Hello, runme!\"\n```", string(blocks[1].Value()))
	assert.Equal(t, "It can have an annotation with a name:", string(blocks[2].Value()))
	assert.Equal(t, "```sh {name=echo first= second=2}\n$ echo \"Hello, runme!\"\n```", string(blocks[3].Value()))

	codeBlocks := blocks.CodeBlocks()
	assert.Equal(t, "echo-hello", codeBlocks[0].Name())
	assert.Equal(t, "echo", codeBlocks[1].Name())
	assert.Equal(t, map[string]string{"name": "echo-hello"}, codeBlocks[0].Attributes())
	assert.Equal(t, map[string]string{"name": "echo", "first": "", "second": "2"}, codeBlocks[1].Attributes())
	assert.Equal(t, "sh", codeBlocks[0].Language())
	assert.Equal(t, "sh", codeBlocks[1].Language())
}

// func TestCodeBlock_MapLines(t *testing.T) {
// 	data := []byte("```" + `sh
// echo 1
// echo 2
// echo 3
// ` + "```")
// 	source := NewSource(data)
// 	block := source.Parse().Blocks().CodeBlocks()[0]
// 	err := block.MapLines(func(s string) (string, error) {
// 		return strings.Split(s, " ")[1], nil
// 	})
// 	require.NoError(t, err)
// 	assert.Equal(t, []string{"1", "2", "3"}, block.Lines())
// }

// func TestMarkdownBlock_Content(t *testing.T) {
// 	data := []byte(`This is a basic snippet with a shell command:

// ` + "```" + `sh
// $ echo "Hello, runme!"
// ` + "```" + `

// It can have an annotation with a name:

// ` + "```" + `sh {name=myname first= second=2}
// $ echo "Hello, runme!"
// ` + "```")
// 	source := NewSource(data)
// 	blocks := source.Parse().Blocks()
// 	assert.Equal(t, "This is a basic snippet with a shell command:", blocks[0].(*MarkdownBlock).Content())
// 	assert.Equal(t, "It can have an annotation with a name:", blocks[2].(*MarkdownBlock).Content())
// }

// func TestMarkdownBlock_Content2(t *testing.T) {
// 	data := []byte(`  > bq 1
//   > bq 2

// 1. Item 1
// 1. Item 2

// ` + "```sh" + `
// echo 1
// echo 2
// ` + "```" + `

//    a
//    b
// `)
// 	source := NewSource(data)
// 	blocks := source.Parse().Blocks()
// 	assert.Equal(t, 4, len(blocks))
// 	assert.Equal(t, "  > bq 1\n  > bq 2", blocks[0].(*MarkdownBlock).Content())
// 	assert.Equal(t, "1. Item 1\n1. Item 2", blocks[1].(*MarkdownBlock).Content())
// 	assert.Equal(t, "```sh\necho 1\necho 2\n```", blocks[2].(*CodeBlock).Content())
// 	assert.Equal(t, "   a\n   b", blocks[3].(*MarkdownBlock).Content())
// }

// func TestMarkdownBlock_Content_Headings(t *testing.T) {
// 	data := []byte(`# ATX Heading

// ## ATX Heading 2

// ###

// Setext 1
// ========

// Setext 2
// --------`)
// 	source := NewSource(data)
// 	blocks := source.Parse().Blocks()
// 	assert.Equal(t, 4, len(blocks))
// 	assert.Equal(t, "# ATX Heading", blocks[0].(*MarkdownBlock).Content())
// 	assert.Equal(t, "## ATX Heading 2", blocks[1].(*MarkdownBlock).Content())
// 	assert.Equal(t, "Setext 1\n========", blocks[2].(*MarkdownBlock).Content())
// 	assert.Equal(t, "Setext 2\n--------", blocks[3].(*MarkdownBlock).Content())
// }

// func TestMarkdownBlock_Content_ThematicBreaks(t *testing.T) {
// 	data := []byte(`Paragraph 1

// ---

// Paragraph 2

// ---

// ` + "```" + `

// ` + "```" + `

// ---

// <p>html</p>`)
// 	source := NewSource(data)
// 	blocks := source.Parse().Blocks()
// 	assert.Equal(t, 7, len(blocks))
// 	assert.Equal(t, "Paragraph 1", blocks[0].(*MarkdownBlock).Content())
// 	assert.Equal(t, "---", blocks[1].(*MarkdownBlock).Content())
// 	assert.Equal(t, "Paragraph 2", blocks[2].(*MarkdownBlock).Content())
// 	assert.Equal(t, "---", blocks[3].(*MarkdownBlock).Content())
// }

// func TestGetContentRange(t *testing.T) {
// 	data := []byte("Start\n\n```\n\n```\nMid\n\n```\n\n```")
// 	source := NewSource(data)
// 	blocks := source.Parse().Blocks()
// 	assert.Equal(t, 4, len(blocks))
// 	start, stop := getContentRange(data, blocks[1].(*CodeBlock).inner)
// 	assert.Equal(t, "```\n\n```", string(data[start:stop]))

// 	start, stop = getContentRange(data, blocks[3].(*CodeBlock).inner)
// 	assert.Equal(t, "```\n\n```", string(data[start:stop]))
// }

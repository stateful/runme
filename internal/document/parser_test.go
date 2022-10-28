package document

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParsedSource_Blocks_nestedFancedCodeBlocks(t *testing.T) {
	data := []byte(`Test involving nested blocks:

1. Install the linkerd CLI

    ` + "```" + `bash
    curl https://run.linkerd.io/install | sh
    ` + "```" + `

1. Install Linkerd2

    ` + "```" + `bash
    linkerd install | kubectl apply -f -
    ` + "```" + `

Final paragraph!

> Code in a block quote:
> ` + "```" + `
> a = b
> ` + "```" + `
> End of the block quote`)
	source := NewSource(data)
	blocks := source.Parse().Blocks()
	assert.Equal(t, 9, len(blocks))
	assert.Equal(t, "Test involving nested blocks:", blocks[0].(*MarkdownBlock).Content())
	assert.Equal(t, "1. Install the linkerd CLI", blocks[1].(*MarkdownBlock).Content())
	assert.Equal(t, []string{"curl https://run.linkerd.io/install | sh"}, blocks[2].(*CodeBlock).Lines())
	assert.Equal(t, "1. Install Linkerd2", blocks[3].(*MarkdownBlock).Content())
	assert.Equal(t, []string{"linkerd install | kubectl apply -f -"}, blocks[4].(*CodeBlock).Lines())
	assert.Equal(t, "Final paragraph!", blocks[5].(*MarkdownBlock).Content())
	assert.Equal(t, "> Code in a block quote:", blocks[6].(*MarkdownBlock).Content())
	assert.Equal(t, []string{"a = b"}, blocks[7].(*CodeBlock).Lines())
	assert.Equal(t, "> End of the block quote", blocks[8].(*MarkdownBlock).Content())
}

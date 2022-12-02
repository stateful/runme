package document

import (
	"testing"

	"github.com/stateful/runme/internal/renderer/cmark"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testDataNested = []byte(`> bq1
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

func TestNode_String(t *testing.T) {
	doc := New(testDataNested, cmark.Render)
	node, _, err := doc.Parse()
	require.NoError(t, err)
	assert.Equal(t, string(testDataNested), node.String())
}

func TestCollectCodeBlocks(t *testing.T) {
	doc := New(testDataNested, cmark.Render)
	node, _, err := doc.Parse()
	require.NoError(t, err)
	codeBlocks := CollectCodeBlocks(node)
	assert.Len(t, codeBlocks, 2)
	assert.Equal(t, "```sh {name=echo first= second=2}\n$ echo \"Hello, runme!\"\n```\n", string(codeBlocks[0].Value()))
	assert.Equal(t, "```sh\necho 1\n```\n", string(codeBlocks[1].Value()))
}

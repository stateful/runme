package document

import (
	"bytes"
	"testing"

	"github.com/stateful/runme/internal/document/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var identityResolverNone = identity.NewResolver(identity.UnspecifiedLifecycleIdentity)

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
	doc := New(testDataNested, identityResolverNone)
	node, err := doc.Root()
	require.NoError(t, err)
	assert.Equal(t, string(testDataNested), node.String())
}

func TestCollectCodeBlocks(t *testing.T) {
	doc := New(testDataNested, identityResolverNone)
	node, err := doc.Root()
	require.NoError(t, err)
	codeBlocks := CollectCodeBlocks(node)
	assert.Len(t, codeBlocks, 2)
	assert.Equal(t, "```sh {name=echo first= second=2}\n$ echo \"Hello, runme!\"\n```\n", string(codeBlocks[0].Value()))
	assert.Equal(t, "```sh\necho 1\n```\n", string(codeBlocks[1].Value()))
}

func TestCodeBlock_Intro(t *testing.T) {
	data := bytes.TrimSpace([]byte(`
` + "```" + `js { name=echo }
console.log("hello world!")
` + "```" + `

This is an intro

` + "```" + `js { name=echo-2 }
console.log("hello world!")
` + "```" + `

`,
	))

	doc := New(data, identityResolverNone)
	node, err := doc.Root()
	require.NoError(t, err)

	blocks := CollectCodeBlocks(node)
	require.Len(t, blocks, 2)
	assert.Equal(t, "", blocks[0].Intro())
	assert.Equal(t, "This is an intro", blocks[1].Intro())
}

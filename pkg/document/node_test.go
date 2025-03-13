package document

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/runmedev/runme/v3/pkg/document/identity"
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
	assert.Len(t, codeBlocks, 3)
	assert.Equal(t, "```sh {name=echo first= second=2}\n$ echo \"Hello, runme!\"\n```\n", string(codeBlocks[1].Value()))
	assert.Equal(t, "```sh\necho 1\n```\n", string(codeBlocks[2].Value()))
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

func TestCodeBlock_Attributes(t *testing.T) {
	testCases := []struct {
		data             string
		expectedLanguage string
	}{
		{
			data: `
			` + "```" + `{"name":"foo"}
			console.log("hello world!")
			`,
			expectedLanguage: "",
		},
		{
			data: `
			` + "```" + ` {"name":"foo"}
			console.log("hello world!")
			`,
			expectedLanguage: "",
		},
		{
			data: `
			` + "```" + `js {"name":"foo"}
			console.log("hello world!")
			`,
			expectedLanguage: "js",
		},
		{
			data: `
			` + "```" + `js {"name": "foo"}
			console.log("hello world!")
			`,
			expectedLanguage: "js",
		},
		{
			data: `
			` + "```" + `{"name": "foo"}
			console.log("hello world!")
			`,
			expectedLanguage: "",
		},
	}

	for _, tc := range testCases {
		b := bytes.TrimSpace([]byte(tc.data))
		doc := New(b, identityResolverNone)
		node, err := doc.Root()
		require.NoError(t, err)

		blocks := CollectCodeBlocks(node)
		require.Len(t, blocks, 1)

		block := blocks[0]
		assert.Len(t, block.attributes.Items, 1)
		assert.Equal(t, tc.expectedLanguage, block.language)
	}
}

func TestCodeBlock_NodeFilter(t *testing.T) {
	source := []byte(`
` + "```" + `sh {"transform":"false"}
echo test1
` + "```" + `

` + "```" + `sh {"transform":"true"}
echo test2
` + "```" + `

` + "```" + `mermaid
graph TD;
A-->B;
A-->C;
B-->D;
C-->D;
` + "```" + `

` + "```" + `mermaid {"ignore":"false"}
graph TD;
A-->B;
A-->C;
B-->D;
C-->D;
` + "```" + `
`)

	doc := New(source, identityResolverNone)
	node, err := doc.Root()
	require.NoError(t, err)

	var all CodeBlocks
	collectCodeBlocks(node, &all)
	require.Len(t, all, 4)

	var filtered CodeBlocks
	collectCodeBlocks(node, &filtered, withExcludeIgnored())
	require.Len(t, filtered, 2)

	first := filtered[0]
	require.Equal(t, "sh", first.Language())
	require.Equal(t, map[string]string(map[string]string{"transform": "true"}), first.Attributes().Items)
	require.Equal(t, []byte("echo test2"), first.Content())

	second := filtered[1]
	require.Equal(t, "mermaid", second.Language())
	require.Equal(t, map[string]string(map[string]string{"ignore": "false"}), second.Attributes().Items)
	require.Equal(t, []byte("graph TD;\nA-->B;\nA-->C;\nB-->D;\nC-->D;"), second.Content())
}

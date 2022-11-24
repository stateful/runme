package document

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParsedSource_buildBlocksTree(t *testing.T) {
	data := []byte(`# Examples

1. Item 1

    ` + "```" + `sh {name=echo first= second=2}
    $ echo "Hello, runme!"
    ` + "```" + `

    Inner paragraph

2. Item 2
`)
	source := NewSource(data)
	parsed := source.Parse()

	nameRes := &nameResolver{
		namesCounter: map[string]int{},
		cache:        map[interface{}]string{},
	}

	tree := &Node{}
	parsed.buildBlocksTree(nameRes, parsed.Root(), tree)
	assert.Len(t, tree.children, 2)
}

// func TestParsedSource_Blocks_nestedFancedCodeBlocks(t *testing.T) {
// 	data := []byte(`Test involving nested blocks:

// 1. Install the linkerd CLI

//     ` + "```" + `bash
//     curl https://run.linkerd.io/install | sh
//     ` + "```" + `

// 1. Install Linkerd2

//     ` + "```" + `bash
//     linkerd install | kubectl apply -f -
//     ` + "```" + `

// Final paragraph!`)
// 	source := NewSource(data)
// 	blocks := source.Parse().Blocks()
// 	assert.Equal(t, 6, len(blocks))
// 	assert.Equal(t, "Test involving nested blocks:", blocks[0].Content())
// 	assert.Equal(t, "1. Install the linkerd CLI", blocks[1].Content())
// 	assert.Equal(t, []string{"curl https://run.linkerd.io/install | sh"}, blocks[2].(*CodeBlock).Lines())
// 	assert.Equal(t, "1. Install Linkerd2", blocks[3].Content())
// 	assert.Equal(t, []string{"linkerd install | kubectl apply -f -"}, blocks[4].(*CodeBlock).Lines())
// 	assert.Equal(t, "Final paragraph!", blocks[5].Content())
// }

// func TestParsedSource_Blocks_nestedFancedCodeBlocksSquashed(t *testing.T) {
// 	// Source: https://github.com/vercel/next.js/blob/canary/contributing/core/developing.md
// 	data := []byte(`1. Install the [GitHub CLI](https://github.com/cli/cli#installation).
// 1. Clone the Next.js repository:
//    ` + "```" + `
//    gh repo clone vercel/next.js
//    ` + "```" + `
// 1. Create a new branch:
//    ` + "```" + `
//    git checkout -b MY_BRANCH_NAME origin/canary
//    ` + "```" + `
// `)
// 	source := NewSource(data)
// 	blocks := source.Parse().Blocks()
// 	assert.Equal(t, 4, len(blocks))
// }

// func TestParsedSource_Blocks_multipleNestedFencedCodeBlocks(t *testing.T) {
// 	data := []byte(`1. Configure your remotes correctly.

//    ` + "```sh" + `
//    git remote -v
//    ` + "```" + `

//    **Results:**

//    ` + "```" + `
//    origin       git@github.com:raisedadead/freeCodeCamp.git (fetch)
//    origin       git@github.com:raisedadead/freeCodeCamp.git (push)
//    upstream     git@github.com:freeCodeCamp/freeCodeCamp.git (fetch)
//    upstream     git@github.com:freeCodeCamp/freeCodeCamp.git (push)
//    ` + "```" + `

// 2. Make sure your main branch is pristine and in sync with the upstream.

// ` + "```sh" + `
//    git checkout main
//    git fetch --all --prune
//    git reset --hard upstream/main
//    ` + "```" + `
// `)
// 	source := NewSource(data)
// 	blocks := source.Parse().Blocks()
// 	assert.Equal(t, 6, len(blocks))
// 	assert.Equal(t, "1. Configure your remotes correctly.", blocks[0].Content())
// 	// Note that the content carries over indentation.
// 	assert.Equal(t, "```sh\n   git remote -v\n   ```", blocks[1].Content())
// 	assert.Equal(t, "   **Results:**", blocks[2].Content())
// 	assert.Equal(t, "```\n   origin       git@github.com:raisedadead/freeCodeCamp.git (fetch)\norigin       git@github.com:raisedadead/freeCodeCamp.git (push)\nupstream     git@github.com:freeCodeCamp/freeCodeCamp.git (fetch)\nupstream     git@github.com:freeCodeCamp/freeCodeCamp.git (push)\n   ```", blocks[3].Content())
// 	assert.Equal(t, "2. Make sure your main branch is pristine and in sync with the upstream.", blocks[4].Content())
// 	assert.Equal(t, "```sh\n   git checkout main\n   git fetch --all --prune\n   git reset --hard upstream/main\n   ```", blocks[5].Content())
// }

// func TestParsedSource_Blocks_nestedBlockquote(t *testing.T) {
// 	data := []byte(`> Code in a block quote:
// > ` + "```" + `
// > a = b
// > ` + "```" + `
// >
// > ` + "```" + `
// > b = c
// > ` + "```" + `
// >
// > End of the block quote`)
// 	source := NewSource(data)
// 	blocks := source.Parse().Blocks()

// 	for idx, b := range blocks {
// 		t.Logf("Block #%d: %s", idx, b.Content())
// 	}

// 	assert.Equal(t, 4, len(blocks))
// 	assert.Equal(t, "> Code in a block quote:", blocks[0].Content())
// 	// Note that the content carries over the block quote marker.
// 	assert.Equal(t, "```\n> a = b\n> ```", blocks[1].Content())
// 	assert.Equal(t, "```\n> b = c\n> ```", blocks[2].Content())
// 	assert.Equal(t, "> End of the block quote", blocks[3].Content())
// }

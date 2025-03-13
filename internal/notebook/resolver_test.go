package notebook

import (
	"context"
	"testing"

	"github.com/go-playground/assert/v2"
	"github.com/stretchr/testify/require"

	parserv1 "github.com/runmedev/runme/v3/pkg/api/gen/proto/go/runme/parser/v1"
	"github.com/runmedev/runme/v3/pkg/document"
	"github.com/runmedev/runme/v3/pkg/document/identity"
)

func TestResolve_GetCellIndexByBlock(t *testing.T) {
	simpleSource := []byte("---\nrunme:\n  id: 01JJDCG2SQSGV0DP55XCR55AYM\n  version: v3\nshell: dagger shell\nterminalRows: 20\n---\n\n# Compose Notebook Pipelines using the Dagger Shell\n\nLet's get upstream artifacts ready. First, compile the Runme kernel binary.\n\n```sh {\"id\":\"01JJDCG2SPRDWGQ1F4Z6EH69EJ\",\"name\":\"KERNEL_BINARY\"}\ngithub.com/purpleclay/daggerverse/golang $(git https://github.com/runmedev/runme | head | tree) |\n  build | \n  file runme\n```\n\nThen, grab the presetup.sh script to provision the build container.\n\n```sh {\"id\":\"01JJDCG2SQSGV0DP55X86EJFSZ\",\"name\":\"PRESETUP\",\"terminalRows\":\"14\"}\ngit https://github.com/stateful/vscode-runme |\n  head |\n  tree |\n  file dagger/scripts/presetup.sh\n```\n\n## Build the Runme VS Code Extension\n\nLet's tie together above's artifacts via their respective cell names to build the Runme VS Code extension.\n\n```sh {\"id\":\"01JJDCG2SQSGV0DP55X8JVYDNR\",\"name\":\"EXTENSION\",\"terminalRows\":\"25\"}\ngithub.com/stateful/vscode-runme |\n  with-remote github.com/stateful/vscode-runme main |\n  with-container $(KERNEL_BINARY) $(PRESETUP) |\n  build-extension GITHUB_TOKEN\n```\n")

	resolver, err := NewResolver(WithSource(simpleSource))
	require.NoError(t, err)

	doc := document.New(simpleSource, identity.NewResolver(identity.DefaultLifecycleIdentity))
	require.NoError(t, err)
	require.NotNil(t, doc)

	node, err := doc.Root()
	require.NoError(t, err)
	require.Len(t, node.Children(), 8)

	expectedValue := []byte("git https://github.com/stateful/vscode-runme |\n  head |\n  tree |\n  file dagger/scripts/presetup.sh")
	expectedIndex := uint32(4)

	block, ok := node.Children()[expectedIndex].Item().(*document.CodeBlock)
	require.True(t, ok)
	require.NotNil(t, block)
	require.Equal(t, expectedValue, block.Content())

	index, err := resolver.GetCellIndexByBlock(block)
	require.NoError(t, err)
	assert.Equal(t, expectedIndex, index)
}

func TestResolveDaggerShell(t *testing.T) {
	ctx := context.Background()

	// fake notebook with dagger shell cells
	daggerShellNotebook := &parserv1.Notebook{
		Cells: []*parserv1.Cell{
			{
				Kind:       parserv1.CellKind_CELL_KIND_CODE,
				LanguageId: "sh",
				Metadata: map[string]string{
					"id":             "01JJDCG2SPRDWGQ1F4Z6EH69EJ",
					"name":           "KERNEL_BINARY",
					"runme.dev/id":   "01JJDCG2SPRDWGQ1F4Z6EH69EJ",
					"runme.dev/name": "KERNEL_BINARY",
				},
				Value: "github.com/purpleclay/daggerverse/golang $(git https://github.com/runmedev/runme | head | tree) |\n  build | \n  file runme",
			},
			{
				Kind:       parserv1.CellKind_CELL_KIND_CODE,
				LanguageId: "sh",
				Metadata: map[string]string{
					"id":             "01JJDCG2SQSGV0DP55X86EJFSZ",
					"name":           "PRESETUP",
					"runme.dev/id":   "01JJDCG2SQSGV0DP55X86EJFSZ",
					"runme.dev/name": "PRESETUP",
				},
				Value: "git https://github.com/stateful/vscode-runme |\n  head |\n  tree |\n  file dagger/scripts/presetup.sh",
			},
			{
				Kind:       parserv1.CellKind_CELL_KIND_CODE,
				LanguageId: "sh",
				Metadata: map[string]string{
					"id":             "01JJDCG2SQSGV0DP55X8JVYDNR",
					"name":           "EXTENSION",
					"runme.dev/id":   "01JJDCG2SQSGV0DP55X8JVYDNR",
					"runme.dev/name": "EXTENSION",
				},
				Value: "github.com/stateful/vscode-runme |\n  with-remote github.com/stateful/vscode-runme main |\n  with-container $(KERNEL_BINARY) $(PRESETUP) |\n  build-extension GITHUB_TOKEN",
			},
		},
		Metadata: map[string]string{
			"runme.dev/frontmatter": "---\nrunme:\n  id: 01JJDCG2SQSGV0DP55XCR55AYM\n  version: v3\nshell: dagger shell\nterminalRows: 20\n---",
		},
	}

	resolver, err := NewResolver(WithNotebook(daggerShellNotebook))
	require.NoError(t, err)

	definition := `KERNEL_BINARY()
{
  github.com/purpleclay/daggerverse/golang $(git https://github.com/runmedev/runme | head | tree) \
    | build \
    | file runme
}
PRESETUP()
{
  git https://github.com/stateful/vscode-runme | head | tree \
    | file dagger/scripts/presetup.sh
}
EXTENSION()
{
  github.com/stateful/vscode-runme | with-remote github.com/stateful/vscode-runme main | with-container $(KERNEL_BINARY) $(PRESETUP) | build-extension GITHUB_TOKEN
}
`

	expectedScripts := []string{
		definition + "KERNEL_BINARY\n",
		definition + "PRESETUP\n",
		definition + "EXTENSION\n",
	}

	for cellIndex, expectedScript := range expectedScripts {
		script, err := resolver.ResolveDaggerShell(ctx, uint32(cellIndex))
		require.NoError(t, err)
		assert.Equal(t, expectedScript, script)
	}
}

func TestResolveDaggerShell_Source(t *testing.T) {
	simpleSource := "---\nshell: dagger shell\n---\n\n```sh {\"name\":\"simple_dagger\",\"terminalRows\":\"18\"}\n### Exported in runme.dev as simple_dagger\ngit github.com/runmedev/runme |\n    head |\n    tree |\n    file examples/README.md\n```\n"

	resolver, err := NewResolver(WithSource([]byte(simpleSource)))
	require.NoError(t, err)

	script, err := resolver.ResolveDaggerShell(context.Background(), uint32(0))
	require.NoError(t, err)

	assert.Equal(t, "simple_dagger()\n{\n  git github.com/runmedev/runme \\\n    | head \\\n    | tree \\\n    | file examples/README.md\n}\nsimple_dagger\n", script)
}

func TestResolveDaggerShell_EmptyRunmeMetadata(t *testing.T) {
	ctx := context.Background()

	// fake notebook with dagger shell cells
	daggerShellNotebook := &parserv1.Notebook{
		Cells: []*parserv1.Cell{
			{
				Kind:       parserv1.CellKind_CELL_KIND_CODE,
				LanguageId: "sh",
				Metadata:   nil,
				Value:      "git github.com/runmedev/runme |\n    head |\n    tree |\n    file examples/README.md",
			},
		},
		Metadata: map[string]string{
			"runme.dev/frontmatter": "---\nshell: dagger shell\n---",
		},
	}

	resolver, err := NewResolver(WithNotebook(daggerShellNotebook))
	require.NoError(t, err)

	stub := `{
  git github.com/runmedev/runme \
    | head \
    | tree \
    | file examples/README.md
}
DAGGER_`

	script, err := resolver.ResolveDaggerShell(ctx, uint32(0))
	require.NoError(t, err)
	require.Contains(t, script, stub)
}

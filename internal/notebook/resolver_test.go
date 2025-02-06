package notebook

import (
	"context"
	"testing"

	"github.com/go-playground/assert/v2"
	parserv1 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/parser/v1"
	"github.com/stretchr/testify/require"
)

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
				Value: "github.com/purpleclay/daggerverse/golang $(git https://github.com/stateful/runme | head | tree) |\n  build | \n  file runme",
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

	resolver := NewNotebookResolver(daggerShellNotebook)
	declaration := `KERNEL_BINARY()
{
  github.com/purpleclay/daggerverse/golang $(git https://github.com/stateful/runme | head | tree) \
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
		declaration + "KERNEL_BINARY\n",
		declaration + "PRESETUP\n",
		declaration + "EXTENSION\n",
	}

	for cellIndex, expectedScript := range expectedScripts {
		script, err := resolver.ResolveDaggerShell(ctx, uint32(cellIndex))
		require.NoError(t, err)
		assert.Equal(t, expectedScript, script)
	}
}

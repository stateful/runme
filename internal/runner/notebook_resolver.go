package runner

import (
	"context"
	"fmt"
	"strconv"

	parserv1 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/parser/v1"
	runnerv1 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/runner/v1"
	"github.com/stateful/runme/v3/pkg/document/editor/editorservice"
	"go.uber.org/zap"
)

type NotebookResolver struct {
	notebook *parserv1.Notebook
	editor   parserv1.ParserServiceServer
}

func NewNotebookResolver(notebook *parserv1.Notebook) *NotebookResolver {

	return &NotebookResolver{
		notebook: notebook,
		editor:   editorservice.NewParserServiceServer(zap.NewNop()),
	}
}

func (r *NotebookResolver) ParseNotebook(context context.Context) (*parserv1.Notebook, error) {
	ser, err := r.editor.Serialize(context, &parserv1.SerializeRequest{Notebook: r.notebook})
	if err != nil {
		return nil, err
	}
	des, err := r.editor.Deserialize(context, &parserv1.DeserializeRequest{Source: ser.Result})
	if err != nil {
		return nil, err
	}

	return des.Notebook, nil
}

func (r *NotebookResolver) ResolveNotebook(context context.Context, target string) (*runnerv1.ResolveNotebookResponse, error) {
	notebook, err := r.ParseNotebook(context)
	if err != nil {
		return nil, err
	}

	script := ""
	if notebook.Frontmatter.Shell != "dagger shell" {
		return &runnerv1.ResolveNotebookResponse{Script: script}, nil
	}

	for _, cell := range notebook.Cells {
		if cell.GetKind() != parserv1.CellKind_CELL_KIND_CODE {
			continue
		}

		languageID := cell.GetLanguageId()
		if languageID != "sh" && languageID != "dagger" {
			continue
		}

		id, okID := cell.Metadata["runme.dev/id"]
		known, okName := cell.Metadata["runme.dev/name"]
		generated := cell.Metadata["runme.dev/nameGenerated"]
		if !okID && !okName {
			continue
		}

		isGenerated, err := strconv.ParseBool(generated)
		if !okName || isGenerated || err != nil {
			known = fmt.Sprintf("FN_%s", id)
		}

		snippet := cell.GetValue()
		script += fmt.Sprintf("%s() {\n\t%s\n}\n", known, snippet)
	}

	script += "\n" + target + "\n"

	return &runnerv1.ResolveNotebookResponse{
		Script: script,
	}, nil
}

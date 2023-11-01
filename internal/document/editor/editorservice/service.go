package editorservice

import (
	"context"

	"github.com/stateful/runme/internal/document/editor"
	parserv1 "github.com/stateful/runme/internal/gen/proto/go/runme/parser/v1"
	"github.com/stateful/runme/internal/idgen"
	"go.uber.org/zap"
	"golang.org/x/exp/constraints"
)

type parserServiceServer struct {
	parserv1.UnimplementedParserServiceServer

	logger *zap.Logger
}

func NewParserServiceServer(logger *zap.Logger) parserv1.ParserServiceServer {
	return &parserServiceServer{logger: logger}
}

func (s *parserServiceServer) Deserialize(_ context.Context, req *parserv1.DeserializeRequest) (*parserv1.DeserializeResponse, error) {
	s.logger.Info("Deserialize", zap.ByteString("source", req.Source[:min(len(req.Source), 64)]))

	notebook, err := editor.Deserialize(req.Source)
	if err != nil {
		s.logger.Info("failed to call Deserialize", zap.Error(err))
		return nil, err
	}

	fmtr, _ := notebook.ParsedFrontmatter()

	cells := make([]*parserv1.Cell, 0, len(notebook.Cells))
	for _, cell := range notebook.Cells {
		var TextRange *parserv1.TextRange
		cellTextRange := cell.TextRange

		if cellTextRange != nil {
			TextRange = &parserv1.TextRange{
				Start: uint32(cellTextRange.Start + notebook.GetContentOffset()),
				End:   uint32(cellTextRange.End + notebook.GetContentOffset()),
			}
		}

		if cell.Kind == editor.CodeKind {
			cell.EnsureId()
		}

		cells = append(cells, &parserv1.Cell{
			Kind:       parserv1.CellKind(cell.Kind),
			Value:      cell.Value,
			LanguageId: cell.LanguageID,
			Metadata:   cell.Metadata,
			TextRange:  TextRange,
		})
	}

	return &parserv1.DeserializeResponse{
		Notebook: &parserv1.Notebook{
			Cells:       cells,
			Metadata:    notebook.Metadata,
			Frontmatter: fmtr.ToParser(),
		},
	}, nil
}

func (s *parserServiceServer) Serialize(_ context.Context, req *parserv1.SerializeRequest) (*parserv1.SerializeResponse, error) {
	s.logger.Info("Serialize")

	cells := make([]*editor.Cell, 0, len(req.Notebook.Cells))
	for _, cell := range req.Notebook.Cells {

		if _, ok := cell.Metadata["id"]; !ok && cell.Kind == parserv1.CellKind_CELL_KIND_CODE {
			cell.Metadata["id"] = idgen.GenerateID()
		}

		cells = append(cells, &editor.Cell{
			Kind:       editor.CellKind(cell.Kind),
			Value:      cell.Value,
			LanguageID: cell.LanguageId,
			Metadata:   cell.Metadata,
		})
	}

	notebook := editor.Notebook{
		Cells:    cells,
		Metadata: req.Notebook.Metadata,
	}

	// notebook.SetFronmatter(document.Frontmatter{
	// 	Runme: document.RunmeMetaData{
	// 		Id:      req.Notebook.Frontmatter.Runme.Id,
	// 		Version: req.Notebook.Frontmatter.Runme.Version,
	// 	},
	// 	Cwd:         req.Notebook.Frontmatter.Cwd,
	// 	Shell:       req.Notebook.Frontmatter.Shell,
	// 	SkipPrompts: req.Notebook.Frontmatter.SkipPrompts,
	// })

	data, err := editor.Serialize(&notebook)

	if err != nil {
		s.logger.Info("failed to call Serialize", zap.Error(err))
		return nil, err
	}
	return &parserv1.SerializeResponse{Result: data}, nil
}

func min[T constraints.Ordered](a, b T) T {
	if a < b {
		return a
	}
	return b
}

package editorservice

import (
	"context"

	"github.com/stateful/runme/internal/document/editor"
	"github.com/stateful/runme/internal/document/identity"
	parserv1 "github.com/stateful/runme/internal/gen/proto/go/runme/parser/v1"
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

	identityResolver := identity.NewResolver(identity.ToLifecycleIdentity(req.Options.Identity))
	notebook, err := editor.Deserialize(req.Source, identityResolver)
	if err != nil {
		s.logger.Info("failed to call Deserialize", zap.Error(err))
		return nil, err
	}

	cells := make([]*parserv1.Cell, 0, len(notebook.Cells))
	for _, cell := range notebook.Cells {
		var tr *parserv1.TextRange

		if cell.TextRange != nil {
			tr = &parserv1.TextRange{
				Start: uint32(cell.TextRange.Start),
				End:   uint32(cell.TextRange.End),
			}
		}

		cells = append(cells, &parserv1.Cell{
			Kind:       parserv1.CellKind(cell.Kind),
			Value:      cell.Value,
			LanguageId: cell.LanguageID,
			Metadata:   cell.Metadata,
			TextRange:  tr,
		})
	}

	var frontmatter *parserv1.Frontmatter

	if notebook.Frontmatter != nil {
		frontmatter = &parserv1.Frontmatter{
			Shell:       notebook.Frontmatter.Shell,
			Cwd:         notebook.Frontmatter.Cwd,
			SkipPrompts: notebook.Frontmatter.SkipPrompts,
		}
	}

	return &parserv1.DeserializeResponse{
		Notebook: &parserv1.Notebook{
			Cells:       cells,
			Metadata:    notebook.Metadata,
			Frontmatter: frontmatter,
		},
	}, nil
}

func (s *parserServiceServer) Serialize(_ context.Context, req *parserv1.SerializeRequest) (*parserv1.SerializeResponse, error) {
	s.logger.Info("Serialize")

	cells := make([]*editor.Cell, 0, len(req.Notebook.Cells))
	for _, cell := range req.Notebook.Cells {
		cells = append(cells, &editor.Cell{
			Kind:       editor.CellKind(cell.Kind),
			Value:      cell.Value,
			LanguageID: cell.LanguageId,
			Metadata:   cell.Metadata,
		})
	}

	data, err := editor.Serialize(&editor.Notebook{
		Cells:    cells,
		Metadata: req.Notebook.Metadata,
	})
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

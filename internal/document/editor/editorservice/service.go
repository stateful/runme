package editorservice

import (
	"context"

	"github.com/stateful/runme/internal/document/editor"
	parserv1 "github.com/stateful/runme/internal/gen/proto/go/runme/parser/v1"
)

type parserServiceServer struct {
	parserv1.UnimplementedParserServiceServer
}

func NewParserServiceServer() parserv1.ParserServiceServer {
	return &parserServiceServer{}
}

func (s *parserServiceServer) Deserialize(_ context.Context, req *parserv1.DeserializeRequest) (*parserv1.DeserializeResponse, error) {
	notebook, err := editor.Deserialize(req.Source)
	if err != nil {
		return nil, err
	}

	cells := make([]*parserv1.Cell, 0, len(notebook.Cells))
	for _, cell := range notebook.Cells {
		cells = append(cells, &parserv1.Cell{
			Kind:       parserv1.CellKind(cell.Kind),
			Value:      cell.Value,
			LanguageId: cell.LanguageID,
			Metadata:   cell.Metadata,
		})
	}

	return &parserv1.DeserializeResponse{
		Notebook: &parserv1.Notebook{
			Cells:    cells,
			Metadata: notebook.Metadata,
		},
	}, nil
}

func (s *parserServiceServer) Serialize(_ context.Context, req *parserv1.SerializeRequest) (*parserv1.SerializeResponse, error) {
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
		return nil, err
	}
	return &parserv1.SerializeResponse{Result: data}, nil
}

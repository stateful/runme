package editorservice

import (
	"context"
	"encoding/base64"
	"strings"

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

		runme := parserv1.FrontmatterRunme{}

		if notebook.Frontmatter.Runme.ID != "" {
			runme.Id = notebook.Frontmatter.Runme.ID
		}

		if notebook.Frontmatter.Runme.Version != "" {
			runme.Version = notebook.Frontmatter.Runme.Version
		}

		if runme.Id != "" || runme.Version != "" {
			frontmatter.Runme = &runme
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
		outputs := make([]*editor.CellOutput, 0, len(cell.Outputs))
		for _, cellOut := range cell.Outputs {
			var outputItems []*editor.CellOutputItem
			for _, item := range cellOut.Items {
				if strings.HasPrefix(item.Mime, "stateful.") {
					continue
				}

				if len(item.Data) <= 0 {
					continue
				}

				dataBase64 := ""
				dataValue := ""
				if !strings.HasPrefix(item.Mime, "image") {
					dataValue = string(item.Data)
				} else {
					dataBase64 = base64.URLEncoding.EncodeToString(item.Data)
				}

				outputItems = append(outputItems, &editor.CellOutputItem{
					Data:  dataBase64,
					Value: dataValue,
					Type:  item.Type,
					Mime:  item.Mime,
				})
			}

			if len(outputItems) <= 0 {
				continue
			}

			var outputProcessInfo *editor.CellOutputProcessInfo
			if cellOut.ProcessInfo != nil {
				outputProcessInfo = &editor.CellOutputProcessInfo{
					ExitReason: &editor.ProcessInfoExitReason{
						Type: cellOut.ProcessInfo.ExitReason.Type,
						Code: cellOut.ProcessInfo.ExitReason.Code,
					},
					Pid: cellOut.ProcessInfo.Pid,
				}
			}

			outputs = append(outputs, &editor.CellOutput{
				Items:       outputItems,
				Metadata:    cellOut.Metadata,
				ProcessInfo: outputProcessInfo,
			})
		}

		var execSummary *editor.CellExecutionSummary
		if cell.ExecutionSummary != nil {
			execSummary = &editor.CellExecutionSummary{}

			if cell.ExecutionSummary.ExecutionOrder != nil {
				execSummary.ExecutionOrder = cell.ExecutionSummary.ExecutionOrder.Value
			}

			if cell.ExecutionSummary.Success != nil {
				execSummary.Success = cell.ExecutionSummary.Success.Value
			}

			if cell.ExecutionSummary.Timing != nil {
				execSummary.Timing = &editor.ExecutionSummaryTiming{
					StartTime: cell.ExecutionSummary.Timing.StartTime.Value,
					EndTime:   cell.ExecutionSummary.Timing.EndTime.Value,
				}
			}
		}

		cells = append(cells, &editor.Cell{
			Kind:             editor.CellKind(cell.Kind),
			Value:            cell.Value,
			LanguageID:       cell.LanguageId,
			Metadata:         cell.Metadata,
			Outputs:          outputs,
			ExecutionSummary: execSummary,
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

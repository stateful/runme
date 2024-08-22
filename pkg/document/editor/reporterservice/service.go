package reporterservice

import (
	"context"

	"go.uber.org/zap"

	parserv1 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/parser/v1"
	reporterv1 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/reporter/v1"
)

type reporterServiceServer struct {
	reporterv1.UnimplementedReporterServiceServer

	logger *zap.Logger
}

func NewReporterServiceServer(logger *zap.Logger) reporterv1.ReporterServiceServer {
	return &reporterServiceServer{logger: logger}
}

func (s *reporterServiceServer) Transform(ctx context.Context, req *reporterv1.TransformRequest) (*reporterv1.TransformResponse, error) {

	var notebook = &parserv1.Notebook{
		Metadata:    req.Notebook.Metadata,
		Frontmatter: req.Notebook.Frontmatter,
		Cells:       make([]*parserv1.Cell, len(req.Notebook.Cells)),
	}

	for i, cell := range req.Notebook.Cells {
		notebook.Cells[i] = &parserv1.Cell{
			Kind:             cell.Kind,
			Value:            cell.Value,
			LanguageId:       cell.LanguageId,
			Metadata:         cell.Metadata,
			TextRange:        cell.TextRange,
			ExecutionSummary: cell.ExecutionSummary,
			Outputs:          make([]*parserv1.CellOutput, len(cell.Outputs)),
		}

		for j, output := range cell.Outputs {
			var filteredItems []*parserv1.CellOutputItem

			for _, item := range output.Items {
				// Only keep stdout items
				if item.Mime == "application/vnd.code.notebook.stdout" {
					filteredItems = append(filteredItems, item)
				}
			}

			notebook.Cells[i].Outputs[j] = &parserv1.CellOutput{
				ProcessInfo: output.ProcessInfo,
				Metadata:    output.Metadata,
				Items:       filteredItems,
			}

		}

	}

	return &reporterv1.TransformResponse{
		Notebook: notebook,
		Extension: &reporterv1.ReporterExtension{
			AutoSave: *req.AutoSave,
			Git: &reporterv1.ReporterGit{
				Repository: *req.Repository,
				Branch:     *req.Branch,
				Commit:     *req.Commit,
			},
			File: &reporterv1.ReporterFile{
				Path:    *req.FilePath,
				Content: req.FileContent,
			},
			Session: &reporterv1.ReporterSession{
				PlainOutput:  req.PlainOutput,
				MaskedOutput: req.MaskedOutput,
			},
			Device: &reporterv1.ReporterDevice{
				MacAddress:     *req.MacAddress,
				Hostname:       *req.Hostname,
				Platform:       *req.Platform,
				Release:        *req.Release,
				Arch:           *req.Arch,
				Vendor:         *req.Vendor,
				Shell:          *req.Shell,
				VsAppHost:      *req.VsAppHost,
				VsAppName:      *req.VsAppName,
				VsAppSessionId: *req.VsAppSessionId,
				VsMachineId:    *req.VsMachineId,
				VsMetadata:     req.VsMetadata,
			},
		},
	}, nil
}

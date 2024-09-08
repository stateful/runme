package reporterservice

import (
	"context"

	"go.uber.org/zap"

	parserv1 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/parser/v1"
	reporterv1alpha1 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/reporter/v1alpha1"
)

type reporterServiceServer struct {
	reporterv1alpha1.UnimplementedReporterServiceServer

	logger *zap.Logger
}

func NewReporterServiceServer(logger *zap.Logger) reporterv1alpha1.ReporterServiceServer {
	return &reporterServiceServer{logger: logger}
}

func (s *reporterServiceServer) Transform(ctx context.Context, req *reporterv1alpha1.TransformRequest) (*reporterv1alpha1.TransformResponse, error) {
	notebook := &parserv1.Notebook{
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

	return &reporterv1alpha1.TransformResponse{
		Notebook: notebook,
		Extension: &reporterv1alpha1.ReporterExtension{
			AutoSave: *req.Extension.AutoSave,
			Git: &reporterv1alpha1.ReporterGit{
				Repository: *req.Extension.Repository,
				Branch:     *req.Extension.Branch,
				Commit:     *req.Extension.Commit,
			},
			File: &reporterv1alpha1.ReporterFile{
				Path:    *req.Extension.FilePath,
				Content: req.Extension.FileContent,
			},
			Session: &reporterv1alpha1.ReporterSession{
				PlainOutput:  req.Extension.PlainOutput,
				MaskedOutput: req.Extension.MaskedOutput,
			},
			Device: &reporterv1alpha1.ReporterDevice{
				MacAddress:     *req.Extension.MacAddress,
				Hostname:       *req.Extension.Hostname,
				Platform:       *req.Extension.Platform,
				Release:        *req.Extension.Release,
				Arch:           *req.Extension.Arch,
				Vendor:         *req.Extension.Vendor,
				Shell:          *req.Extension.Shell,
				VsAppHost:      *req.Extension.VsAppHost,
				VsAppName:      *req.Extension.VsAppName,
				VsAppSessionId: *req.Extension.VsAppSessionId,
				VsMachineId:    *req.Extension.VsMachineId,
				VsMetadata:     req.Extension.VsMetadata,
			},
		},
	}, nil
}

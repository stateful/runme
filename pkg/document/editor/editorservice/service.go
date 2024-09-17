package editorservice

import (
	"context"
	"encoding/base64"
	"strings"

	"go.uber.org/zap"

	parserv1 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/parser/v1"
	"github.com/stateful/runme/v3/pkg/document"
	"github.com/stateful/runme/v3/pkg/document/editor"
	"github.com/stateful/runme/v3/pkg/document/identity"
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

	identityResolver := identity.NewResolver(fromProtoDeserializeReqOptionsToLifecycleIdentity(req.Options))
	notebook, err := editor.Deserialize(req.Source, editor.Options{LoggerInstance: s.logger, IdentityResolver: identityResolver})
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
			Shell:        notebook.Frontmatter.Shell,
			Cwd:          notebook.Frontmatter.Cwd,
			SkipPrompts:  notebook.Frontmatter.SkipPrompts,
			Category:     notebook.Frontmatter.Category,
			Tag:          notebook.Frontmatter.Tag,
			TerminalRows: notebook.Frontmatter.TerminalRows,
		}

		runme := parserv1.FrontmatterRunme{}

		if notebook.Frontmatter.Runme != nil {
			if notebook.Frontmatter.Runme.ID != "" {
				runme.Id = notebook.Frontmatter.Runme.ID
			}

			if notebook.Frontmatter.Runme.Version != "" {
				runme.Version = notebook.Frontmatter.Runme.Version
			}

			if notebook.Frontmatter.Runme.Session != nil && notebook.Frontmatter.Runme.Session.ID != "" {
				runme.Session = &parserv1.RunmeSession{
					Id: notebook.Frontmatter.Runme.Session.ID,
					Document: &parserv1.RunmeSessionDocument{
						RelativePath: notebook.Frontmatter.Runme.Document.RelativePath,
					},
				}
			}
		}

		if runme.Id != "" || runme.Version != "" || runme.Session != nil {
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
		outputs := s.serializeCellOutputs(cell, req.Options)
		executionSummary := s.serializeCellExecutionSummary(cell, req.Options)

		cells = append(cells, &editor.Cell{
			Kind:             editor.CellKind(cell.Kind),
			Value:            cell.Value,
			LanguageID:       cell.LanguageId,
			Metadata:         cell.Metadata,
			Outputs:          outputs,
			ExecutionSummary: executionSummary,
		})
	}

	var outputMetadata *document.RunmeMetadata
	if req.Options != nil && req.Options.Session != nil {
		relativePath := ""
		if req.Options.Session.Document != nil {
			relativePath = req.Options.Session.Document.GetRelativePath()
		}

		outputMetadata = &document.RunmeMetadata{
			Session: &document.RunmeMetadataSession{
				ID: req.Options.Session.GetId(),
			},
			Document: &document.RunmeMetadataDocument{
				RelativePath: relativePath,
			},
		}

	}

	data, err := editor.Serialize(&editor.Notebook{
		Cells:    cells,
		Metadata: req.Notebook.Metadata,
	}, outputMetadata, editor.Options{LoggerInstance: s.logger})
	if err != nil {
		s.logger.Info("failed to call Serialize", zap.Error(err))
		return nil, err
	}

	return &parserv1.SerializeResponse{Result: data}, nil
}

func (*parserServiceServer) serializeCellExecutionSummary(cell *parserv1.Cell, options *parserv1.SerializeRequestOptions) *editor.CellExecutionSummary {
	if options == nil || options.Outputs == nil || !options.Outputs.GetSummary() {
		return nil
	}

	if cell.ExecutionSummary == nil {
		return nil
	}

	execSummary := &editor.CellExecutionSummary{}

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

	return execSummary
}

func (*parserServiceServer) serializeCellOutputs(cell *parserv1.Cell, options *parserv1.SerializeRequestOptions) []*editor.CellOutput {
	outputs := make([]*editor.CellOutput, 0, len(cell.Outputs))

	if options == nil || options.Outputs == nil || !options.Outputs.GetEnabled() {
		return outputs
	}

	for _, cellOutput := range cell.Outputs {
		var outputItems []*editor.CellOutputItem
		for _, item := range cellOutput.Items {
			if strings.HasPrefix(item.Mime, "stateful.") {
				continue
			}

			if cellOutput.ProcessInfo == nil && len(item.Data) <= 0 {
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
		if cellOutput.ProcessInfo != nil {
			exitReason := &editor.ProcessInfoExitReason{
				Type: cellOutput.ProcessInfo.ExitReason.Type,
			}
			if cellOutput.ProcessInfo.ExitReason.Code != nil {
				exitReason.Code = cellOutput.ProcessInfo.ExitReason.Code.Value
			}
			outputProcessInfo = &editor.CellOutputProcessInfo{
				ExitReason: exitReason,
			}
			if cellOutput.ProcessInfo.Pid != nil {
				outputProcessInfo.Pid = cellOutput.ProcessInfo.Pid.Value
			}
		}

		outputs = append(outputs, &editor.CellOutput{
			Items:       outputItems,
			Metadata:    cellOutput.Metadata,
			ProcessInfo: outputProcessInfo,
		})
	}
	return outputs
}

func fromProtoDeserializeReqOptionsToLifecycleIdentity(opt *parserv1.DeserializeRequestOptions) identity.LifecycleIdentity {
	var idt parserv1.RunmeIdentity

	if opt != nil {
		idt = opt.Identity
	}

	switch idt {
	case parserv1.RunmeIdentity_RUNME_IDENTITY_ALL:
		return identity.AllLifecycleIdentity
	case parserv1.RunmeIdentity_RUNME_IDENTITY_DOCUMENT:
		return identity.DocumentLifecycleIdentity
	case parserv1.RunmeIdentity_RUNME_IDENTITY_CELL:
		return identity.CellLifecycleIdentity
	case parserv1.RunmeIdentity_RUNME_IDENTITY_UNSPECIFIED:
		return identity.UnspecifiedLifecycleIdentity
	default:
		return identity.DefaultLifecycleIdentity
	}
}

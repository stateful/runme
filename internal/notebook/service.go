package notebook

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	notebookv1alpha1 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/notebook/v1alpha1"
	runnerv1 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/runner/v1"
)

type notebookService struct {
	notebookv1alpha1.UnimplementedNotebookServiceServer

	logger *zap.Logger
}

func newNotebookService(logger *zap.Logger) *notebookService {
	return &notebookService{
		logger: logger,
	}
}

func NewNotebookService(logger *zap.Logger) notebookv1alpha1.NotebookServiceServer {
	return newNotebookService(logger)
}

func (r *notebookService) ResolveNotebook(ctx context.Context, req *notebookv1alpha1.ResolveNotebookRequest) (*notebookv1alpha1.ResolveNotebookResponse, error) {
	r.logger.Info("running ResolveNotebook in notebookService")
	commandMode := req.GetCommandMode()
	if commandMode != runnerv1.CommandMode_COMMAND_MODE_DAGGER {
		return nil, status.Error(codes.Unimplemented, "command mode is not supported")
	}

	notebook := req.GetNotebook()
	if notebook == nil {
		return nil, status.Error(codes.InvalidArgument, "notebook is required")
	}

	cellIndex := req.GetCellIndex()
	if cellIndex == nil {
		return nil, status.Error(codes.InvalidArgument, "cell index is required")
	}

	resolver, err := NewResolver(WithNotebook(notebook))
	if err != nil {
		r.logger.Error("failed to create notebook resolver", zap.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	script, err := resolver.ResolveDaggerShell(ctx, cellIndex.GetValue())
	if err != nil {
		r.logger.Error("failed to resolve dagger shell", zap.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &notebookv1alpha1.ResolveNotebookResponse{Script: script}, nil
}

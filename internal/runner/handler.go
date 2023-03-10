package runner

import (
	"context"

	"github.com/bufbuild/connect-go"
	v1 "github.com/stateful/runme/internal/gen/proto/go/runme/runner/v1"
	"github.com/stateful/runme/internal/gen/proto/go/runme/runner/v1/runnerv1connect"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type runnerServiceHandler struct {
	service *runnerService
}

func NewRunnerServiceHandler(logger *zap.Logger) (runnerv1connect.RunnerServiceHandler, error) {
	service, err := newRunnerService(logger)
	if err != nil {
		return nil, err
	}

	return &runnerServiceHandler{
		service: service,
	}, nil
}

func (h *runnerServiceHandler) CreateSession(ctx context.Context, req *connect.Request[v1.CreateSessionRequest]) (*connect.Response[v1.CreateSessionResponse], error) {
	resp, err := h.service.CreateSession(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (h *runnerServiceHandler) GetSession(ctx context.Context, req *connect.Request[v1.GetSessionRequest]) (*connect.Response[v1.GetSessionResponse], error) {
	resp, err := h.service.GetSession(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (h *runnerServiceHandler) ListSessions(ctx context.Context, req *connect.Request[v1.ListSessionsRequest]) (*connect.Response[v1.ListSessionsResponse], error) {
	resp, err := h.service.ListSessions(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (h *runnerServiceHandler) DeleteSession(ctx context.Context, req *connect.Request[v1.DeleteSessionRequest]) (*connect.Response[v1.DeleteSessionResponse], error) {
	resp, err := h.service.DeleteSession(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (h *runnerServiceHandler) Execute(ctx context.Context, stream *connect.BidiStream[v1.ExecuteRequest, v1.ExecuteResponse]) error {
	return status.Error(codes.Unimplemented, "Execute is not implemented")
}

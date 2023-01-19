package kernel

import (
	"context"
	"io"

	"github.com/bufbuild/connect-go"
	v1 "github.com/stateful/runme/internal/gen/proto/go/runme/kernel/v1"
	"github.com/stateful/runme/internal/gen/proto/go/runme/kernel/v1/kernelv1connect"
	"go.uber.org/zap"
)

type kernelServiceHandler struct {
	server *kernelServiceServer
}

func NewKernelServiceHandler(logger *zap.Logger) kernelv1connect.KernelServiceHandler {
	return &kernelServiceHandler{
		server: newKernelServiceServer(logger),
	}
}

var _ kernelv1connect.KernelServiceHandler = (*kernelServiceHandler)(nil)

func (h *kernelServiceHandler) PostSession(ctx context.Context, req *connect.Request[v1.PostSessionRequest]) (*connect.Response[v1.PostSessionResponse], error) {
	resp, err := h.server.PostSession(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (h *kernelServiceHandler) DeleteSession(ctx context.Context, req *connect.Request[v1.DeleteSessionRequest]) (*connect.Response[v1.DeleteSessionResponse], error) {
	resp, err := h.server.DeleteSession(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (h *kernelServiceHandler) ListSessions(ctx context.Context, req *connect.Request[v1.ListSessionsRequest]) (*connect.Response[v1.ListSessionsResponse], error) {
	resp, err := h.server.ListSessions(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (h *kernelServiceHandler) Execute(ctx context.Context, req *connect.Request[v1.ExecuteRequest]) (*connect.Response[v1.ExecuteResponse], error) {
	resp, err := h.server.Execute(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (h *kernelServiceHandler) ExecuteStream(ctx context.Context, req *connect.Request[v1.ExecuteRequest], srv *connect.ServerStream[v1.ExecuteResponse]) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	resp := make(chan *v1.ExecuteResponse)
	errC := make(chan error, 2)
	defer close(errC)

	go func() {
		for msg := range resp {
			if err := srv.Send(msg); err != nil {
				errC <- err
				cancel() // interrupt executeStream()
				return
			}
		}
	}()

	go func() {
		errC <- h.server.executeStream(ctx, req.Msg, resp)
	}()

	return <-errC
}

func (h *kernelServiceHandler) Input(ctx context.Context, req *connect.Request[v1.InputRequest]) (*connect.Response[v1.InputResponse], error) {
	resp, err := h.server.Input(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (h *kernelServiceHandler) Output(ctx context.Context, req *connect.Request[v1.OutputRequest], srv *connect.ServerStream[v1.OutputResponse]) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	resp := make(chan *v1.OutputResponse)
	errC := make(chan error, 2)
	defer close(errC)

	go func() {
		for msg := range resp {
			if err := srv.Send(msg); err != nil {
				errC <- err
				cancel() // interrupt output()
				return
			}
		}
	}()

	go func() {
		err := h.server.output(ctx, req.Msg, resp)
		if err != io.EOF {
			errC <- err
		}
	}()

	return <-errC
}

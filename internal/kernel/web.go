package kernel

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/bufbuild/connect-go"
	v1 "github.com/stateful/runme/internal/gen/proto/go/runme/kernel/v1"
	"github.com/stateful/runme/internal/gen/proto/go/runme/kernel/v1/kernelv1connect"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type kernelServiceHandler struct {
	kernel *kernel
	logger *zap.Logger
}

func NewKernelServiceHandler(logger *zap.Logger) kernelv1connect.KernelServiceHandler {
	return &kernelServiceHandler{
		kernel: &kernel{},
		logger: logger,
	}
}

var _ kernelv1connect.KernelServiceHandler = (*kernelServiceHandler)(nil)

func (h *kernelServiceHandler) PostSession(ctx context.Context, req *connect.Request[v1.PostSessionRequest]) (*connect.Response[v1.PostSessionResponse], error) {
	prompt := []byte(req.Msg.Prompt)
	if len(prompt) == 0 {
		var err error
		prompt, err = DetectPrompt(req.Msg.CommandName)
		h.logger.Info("detected prompt", zap.Error(err), zap.ByteString("prompt", prompt))
		if err != nil {
			return nil, fmt.Errorf("failed to detect prompt: %w", err)
		}
	}

	s, err := NewSession(prompt, req.Msg.CommandName, req.Msg.RawOutput, h.logger)
	if err != nil {
		return nil, err
	}

	h.kernel.AddSession(s)

	return connect.NewResponse(&v1.PostSessionResponse{
		Session: &v1.Session{Id: s.id},
	}), nil
}

func (h *kernelServiceHandler) DeleteSession(ctx context.Context, req *connect.Request[v1.DeleteSessionRequest]) (*connect.Response[v1.DeleteSessionResponse], error) {
	session := h.kernel.FindSession(req.Msg.SessionId)
	if session == nil {
		return nil, errors.New("session not found")
	}

	if err := session.Destroy(); err != nil {
		return nil, err
	}

	h.kernel.DeleteSession(session)

	return connect.NewResponse(&v1.DeleteSessionResponse{}), nil
}

func (h *kernelServiceHandler) ListSessions(ctx context.Context, req *connect.Request[v1.ListSessionsRequest]) (*connect.Response[v1.ListSessionsResponse], error) {
	sessions := h.kernel.Sessions()
	resp := v1.ListSessionsResponse{
		Sessions: make([]*v1.Session, len(sessions)),
	}
	for idx, s := range sessions {
		resp.Sessions[idx] = &v1.Session{
			Id: s.ID(),
		}
	}
	return connect.NewResponse(&resp), nil
}

func (h *kernelServiceHandler) Execute(ctx context.Context, req *connect.Request[v1.ExecuteRequest], srv *connect.ServerStream[v1.ExecuteResponse]) error {
	session := h.kernel.FindSession(req.Msg.SessionId)
	if session == nil {
		return errors.New("session not found")
	}

	buf := new(bytes.Buffer)
	execDone := make(chan struct{})
	g, _ := errgroup.WithContext(ctx)

	g.Go(func() error {
		for {
			p := make([]byte, 1024)
			n, rerr := buf.Read(p)
			h.logger.Info("read output from command", zap.ByteString("data", p[:n]), zap.Error(rerr))
			if rerr != nil && rerr != io.EOF {
				return rerr
			}
			if rerr == io.EOF {
				select {
				case <-time.After(time.Millisecond * 200):
					continue
				case <-execDone:
					return nil
				}
			}

			err := srv.Send(&v1.ExecuteResponse{Stdout: p[:n]})
			if err != nil {
				return err
			}
		}
	})

	g.Go(func() error {
		// TODO: investigate exactly what's happening.
		// The problem is that without copying, req.Command
		// is later changed from its original value.
		// gRPC SDK reuses byte arrays?
		command := make([]byte, len(req.Msg.Command))
		copy(command, []byte(req.Msg.Command))

		exitCode, err := session.Execute(command, buf)
		close(execDone)
		if err != nil {
			return err
		}

		data := buf.Bytes()
		h.logger.Info("finished executing command", zap.Int("code", exitCode), zap.ByteString("data", data), zap.Error(err))
		return srv.Send(&v1.ExecuteResponse{
			Stdout:   data,
			ExitCode: wrapperspb.UInt32(uint32(exitCode)),
		})
	})

	return g.Wait()
}

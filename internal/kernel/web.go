package kernel

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/bufbuild/connect-go"
	"github.com/pkg/errors"
	v1 "github.com/stateful/runme/internal/gen/proto/go/runme/kernel/v1"
	"github.com/stateful/runme/internal/gen/proto/go/runme/kernel/v1/kernelv1connect"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type kernelServiceHandler struct {
	sessions *sessionsContainer
	logger   *zap.Logger
}

func NewKernelServiceHandler(logger *zap.Logger) kernelv1connect.KernelServiceHandler {
	return &kernelServiceHandler{
		sessions: &sessionsContainer{},
		logger:   logger,
	}
}

var _ kernelv1connect.KernelServiceHandler = (*kernelServiceHandler)(nil)

func (h *kernelServiceHandler) PostSession(ctx context.Context, req *connect.Request[v1.PostSessionRequest]) (*connect.Response[v1.PostSessionResponse], error) {
	promptStr := req.Msg.Prompt
	if promptStr == "" {
		prompt, err := DetectPrompt(req.Msg.Command)
		h.logger.Info("detected prompt", zap.Error(err), zap.ByteString("prompt", prompt))
		if err != nil {
			return nil, fmt.Errorf("failed to detect prompt: %w", err)
		}
	}

	s, data, err := newSession(req.Msg.Command, promptStr, h.logger)
	if err != nil {
		return nil, err
	}

	h.sessions.AddSession(s)

	return connect.NewResponse(&v1.PostSessionResponse{
		Session:   &v1.Session{Id: s.id},
		IntroData: data,
	}), nil
}

func (h *kernelServiceHandler) DeleteSession(ctx context.Context, req *connect.Request[v1.DeleteSessionRequest]) (*connect.Response[v1.DeleteSessionResponse], error) {
	session := h.sessions.FindSession(req.Msg.SessionId)
	if session == nil {
		return nil, errors.New("session not found")
	}

	h.sessions.DeleteSession(session)

	if err := session.Close(); err != nil {
		return nil, err
	}

	return connect.NewResponse(&v1.DeleteSessionResponse{}), nil
}

func (h *kernelServiceHandler) ListSessions(ctx context.Context, req *connect.Request[v1.ListSessionsRequest]) (*connect.Response[v1.ListSessionsResponse], error) {
	sessions := h.sessions.Sessions()
	resp := v1.ListSessionsResponse{
		Sessions: make([]*v1.Session, len(sessions)),
	}
	for idx, s := range sessions {
		resp.Sessions[idx] = &v1.Session{
			Id: s.id,
		}
	}
	return connect.NewResponse(&resp), nil
}

func (h *kernelServiceHandler) Execute(ctx context.Context, req *connect.Request[v1.ExecuteRequest]) (*connect.Response[v1.ExecuteResponse], error) {
	session := h.sessions.FindSession(req.Msg.SessionId)
	if session == nil {
		return nil, errors.New("session not found")
	}

	data, exitCode, err := session.Execute(req.Msg.Command, time.Minute)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&v1.ExecuteResponse{
		Data:     data,
		ExitCode: wrapperspb.UInt32(uint32(exitCode)),
	}), nil
}

func (h *kernelServiceHandler) ExecuteStream(ctx context.Context, req *connect.Request[v1.ExecuteRequest], srv *connect.ServerStream[v1.ExecuteResponse]) error {
	session := h.sessions.FindSession(req.Msg.SessionId)
	if session == nil {
		return errors.New("session not found")
	}

	chunks := make(chan []byte)
	defer close(chunks)
	errC := make(chan error, 1)

	go func() {
		defer close(errC)
		for line := range chunks {
			err := srv.Send(&v1.ExecuteResponse{
				Data: line,
			})
			if err != nil {
				// Propagate only the first error.
				select {
				case errC <- errors.Wrap(err, "failed to write to writer"):
				default:
				}
			}
		}
	}()

	exitCode, err := session.ExecuteWithChannel(req.Msg.Command, time.Minute, chunks)
	if err != nil {
		return err
	}

	return srv.Send(&v1.ExecuteResponse{
		ExitCode: wrapperspb.UInt32(uint32(exitCode)),
	})
}

func (h *kernelServiceHandler) Input(ctx context.Context, req *connect.Request[v1.InputRequest]) (*connect.Response[v1.InputResponse], error) {
	session := h.sessions.FindSession(req.Msg.SessionId)
	if session == nil {
		return nil, errors.New("session not found")
	}
	if err := session.Send(req.Msg.Data); err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1.InputResponse{}), nil
}

func (h *kernelServiceHandler) Output(ctx context.Context, req *connect.Request[v1.OutputRequest], srv *connect.ServerStream[v1.OutputResponse]) error {
	session := h.sessions.FindSession(req.Msg.SessionId)
	if session == nil {
		return errors.New("session not found")
	}

	execDone := make(chan struct{})
	g, _ := errgroup.WithContext(ctx)

	g.Go(func() error {
		for {
			p := make([]byte, 1024)
			n, rerr := session.Read(p)
			h.logger.Info("read output from session", zap.ByteString("data", p[:n]), zap.Error(rerr))
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

			err := srv.Send(&v1.OutputResponse{Data: p[:n]})
			if err != nil {
				return err
			}
		}
	})

	return nil
}

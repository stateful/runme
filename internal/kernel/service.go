package kernel

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/pkg/errors"
	kernelv1 "github.com/stateful/runme/internal/gen/proto/go/runme/kernel/v1"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func NewKernelServiceServer(logger *zap.Logger) kernelv1.KernelServiceServer {
	return newKernelServiceServer(logger)
}

func newKernelServiceServer(logger *zap.Logger) *kernelServiceServer {
	return &kernelServiceServer{
		sessions: &sessionsContainer{},
		logger:   logger,
	}
}

type kernelServiceServer struct {
	kernelv1.UnimplementedKernelServiceServer

	sessions *sessionsContainer
	logger   *zap.Logger
}

func (s *kernelServiceServer) PostSession(ctx context.Context, req *kernelv1.PostSessionRequest) (*kernelv1.PostSessionResponse, error) {
	promptStr := req.Prompt
	if promptStr == "" {
		prompt, err := DetectPrompt(req.Command)
		s.logger.Info("detected prompt", zap.Error(err), zap.ByteString("prompt", prompt))
		if err != nil {
			return nil, fmt.Errorf("failed to detect prompt: %w", err)
		}
	}

	session, data, err := newSession(req.Command, promptStr, s.logger)
	if err != nil {
		return nil, err
	}

	s.sessions.AddSession(session)

	return &kernelv1.PostSessionResponse{
		Session:   &kernelv1.Session{Id: session.id},
		IntroData: data,
	}, nil
}

func (s *kernelServiceServer) DeleteSession(ctx context.Context, req *kernelv1.DeleteSessionRequest) (*kernelv1.DeleteSessionResponse, error) {
	session := s.sessions.FindSession(req.SessionId)
	if session == nil {
		return nil, errors.New("session not found")
	}

	s.sessions.DeleteSession(session)

	if err := session.Close(); err != nil {
		return nil, err
	}

	return nil, errors.New("session does not exist")
}

func (s *kernelServiceServer) ListSessions(ctx context.Context, req *kernelv1.ListSessionsRequest) (*kernelv1.ListSessionsResponse, error) {
	sessions := s.sessions.Sessions()
	resp := kernelv1.ListSessionsResponse{
		Sessions: make([]*kernelv1.Session, len(sessions)),
	}
	for idx, s := range sessions {
		resp.Sessions[idx] = &kernelv1.Session{
			Id: s.ID(),
		}
	}
	return &resp, nil
}

func (s *kernelServiceServer) Execute(ctx context.Context, req *kernelv1.ExecuteRequest) (*kernelv1.ExecuteResponse, error) {
	session := s.sessions.FindSession(req.SessionId)
	if session == nil {
		return nil, errors.New("session not found")
	}

	data, exitCode, err := session.Execute(req.Command, time.Second*10)
	if err != nil {
		return nil, err
	}

	return &kernelv1.ExecuteResponse{
		Data:     data,
		ExitCode: wrapperspb.UInt32(uint32(exitCode)),
	}, nil
}

func (s *kernelServiceServer) executeStream(
	_ context.Context, // TODO: pass ctx to ExecuteWithChannel()
	req *kernelv1.ExecuteRequest,
	resp chan<- *kernelv1.ExecuteResponse,
) error {
	defer close(resp)

	session := s.sessions.FindSession(req.SessionId)
	if session == nil {
		return errors.New("session not found")
	}

	chunks := make(chan []byte, 10)
	defer close(chunks)

	go func() {
		for data := range chunks {
			resp <- &kernelv1.ExecuteResponse{
				Data: data,
			}
		}
	}()

	exitCode, err := session.ExecuteWithChannel(req.Command, time.Minute, chunks)
	if err != nil {
		return err
	}
	resp <- &kernelv1.ExecuteResponse{ExitCode: wrapperspb.UInt32(uint32(exitCode))}
	return nil
}

func (s *kernelServiceServer) ExecuteStream(req *kernelv1.ExecuteRequest, srv kernelv1.KernelService_ExecuteStreamServer) error {
	ctx, cancel := context.WithCancel(srv.Context())
	defer cancel()

	resp := make(chan *kernelv1.ExecuteResponse)
	done := make(chan struct{})
	var rErr error
	go func() {
		defer close(done)
		for msg := range resp {
			rErr = srv.Send(msg)
			if rErr != nil {
				rErr = errors.WithStack(rErr)
				cancel() // interrupt executeStream()
				return
			}
		}
	}()

	if err := s.executeStream(ctx, req, resp); err != nil {
		return err
	}
	<-done
	return rErr
}

func (s *kernelServiceServer) Input(ctx context.Context, req *kernelv1.InputRequest) (*kernelv1.InputResponse, error) {
	session := s.sessions.FindSession(req.SessionId)
	if session == nil {
		return nil, errors.New("session not found")
	}
	if err := session.Send(req.Data); err != nil {
		return nil, err
	}
	return &kernelv1.InputResponse{}, nil
}

func (s *kernelServiceServer) output(
	_ context.Context,
	req *kernelv1.OutputRequest,
	resp chan<- *kernelv1.OutputResponse,
) error {
	defer close(resp)

	session := s.sessions.FindSession(req.SessionId)
	if session == nil {
		return errors.New("session not found")
	}

	for {
		p := make([]byte, 1024)
		n, rerr := session.Read(p)
		if rerr != nil {
			return rerr
		}
		if n == 0 {
			continue
		}
		resp <- &kernelv1.OutputResponse{Data: p[:n]}
	}
}

func (s *kernelServiceServer) Output(req *kernelv1.OutputRequest, srv kernelv1.KernelService_OutputServer) error {
	ctx, cancel := context.WithCancel(srv.Context())
	defer cancel()

	resp := make(chan *kernelv1.OutputResponse)
	done := make(chan struct{})
	var rErr error
	go func() {
		defer close(done)
		for msg := range resp {
			rErr = srv.Send(msg)
			if rErr != nil {
				rErr = errors.WithStack(rErr)
				cancel() // interrupt output()
				return
			}
		}
	}()

	if err := s.output(ctx, req, resp); err != nil && err != io.EOF {
		return err
	}
	<-done
	return rErr
}

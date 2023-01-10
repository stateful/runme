package kernel

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	kernelv1 "github.com/stateful/runme/internal/gen/proto/go/runme/kernel/v1"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func NewKernelServiceServer(logger *zap.Logger) kernelv1.KernelServiceServer {
	return &kernelServiceServer{logger: logger}
}

type kernelServiceServer struct {
	kernelv1.UnimplementedKernelServiceServer

	kernel *kernel
	logger *zap.Logger
}

func (m *kernelServiceServer) PostSession(ctx context.Context, req *kernelv1.PostSessionRequest) (*kernelv1.PostSessionResponse, error) {
	prompt := []byte(req.Prompt)
	if len(prompt) == 0 {
		var err error
		prompt, err = DetectPrompt(req.CommandName)
		m.logger.Info("detected prompt", zap.Error(err), zap.ByteString("prompt", prompt))
		if err != nil {
			return nil, fmt.Errorf("failed to detect prompt: %w", err)
		}
	}

	s, err := NewSession(prompt, req.CommandName, req.RawOutput, m.logger)
	if err != nil {
		return nil, err
	}

	m.kernel.AddSession(s)

	return &kernelv1.PostSessionResponse{
		Session: &kernelv1.Session{Id: s.id},
	}, nil
}

func (m *kernelServiceServer) DeleteSession(ctx context.Context, req *kernelv1.DeleteSessionRequest) (*kernelv1.DeleteSessionResponse, error) {
	session := m.kernel.FindSession(req.SessionId)
	if session == nil {
		return nil, errors.New("session not found")
	}

	if err := session.Destroy(); err != nil {
		return nil, err
	}

	m.kernel.DeleteSession(session)

	return nil, errors.New("session does not exist")
}

func (m *kernelServiceServer) ListSessions(ctx context.Context, req *kernelv1.ListSessionsRequest) (*kernelv1.ListSessionsResponse, error) {
	sessions := m.kernel.Sessions()
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

func (m *kernelServiceServer) Execute(req *kernelv1.ExecuteRequest, srv kernelv1.KernelService_ExecuteServer) error {
	session := m.kernel.FindSession(req.SessionId)
	if session == nil {
		return errors.New("session not found")
	}

	buf := new(bytes.Buffer)
	execDone := make(chan struct{})
	g, _ := errgroup.WithContext(srv.Context())

	g.Go(func() error {
		// Stream up to 4KB of data per chunk. If there is no data available,
		// check every 200 ms.
		for {
			p := make([]byte, 4096)
			n, rerr := buf.Read(p)
			m.logger.Info("read output from command", zap.ByteString("data", p[:n]), zap.Error(rerr))
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

			err := srv.Send(&kernelv1.ExecuteResponse{Stdout: p[:n]})
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
		command := make([]byte, len(req.Command))
		copy(command, req.Command)

		exitCode, err := session.Execute(command, buf)
		close(execDone)
		if err != nil {
			return err
		}

		data := buf.Bytes()
		m.logger.Info("finished executing command", zap.Int("code", exitCode), zap.ByteString("data", data), zap.Error(err))
		return srv.Send(&kernelv1.ExecuteResponse{
			Stdout:   data,
			ExitCode: wrapperspb.UInt32(uint32(exitCode)),
		})
	})

	return g.Wait()
}

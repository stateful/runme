package kernel

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	kernelv1 "github.com/stateful/runme/internal/gen/proto/go/kernel/v1"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func NewKernelServiceServer(logger *zap.Logger) kernelv1.KernelServiceServer {
	return &kernelServiceServer{logger: logger}
}

type kernelServiceServer struct {
	kernelv1.UnimplementedKernelServiceServer

	sessions []*Session
	logger   *zap.Logger
}

func (m *kernelServiceServer) PostSession(ctx context.Context, req *kernelv1.PostSessionRequest) (*kernelv1.PostSessionResponse, error) {
	prompt := []byte(req.Prompt)
	if len(prompt) == 0 {
		var err error
		prompt, err = DetectPrompt(req.CommandName)
		if err != nil {
			return nil, fmt.Errorf("failed to detect prompt: %w", err)
		}
	}

	s, err := NewSession(prompt, req.CommandName, m.logger)
	if err != nil {
		return nil, err
	}

	m.sessions = append(m.sessions, s)

	return &kernelv1.PostSessionResponse{
		Session: &kernelv1.Session{Id: s.id},
	}, nil
}

func (m *kernelServiceServer) DeleteSession(ctx context.Context, req *kernelv1.DeleteSessionRequest) (*kernelv1.DeleteSessionResponse, error) {
	for idx, s := range m.sessions {
		if s.id == req.SessionId {
			if err := s.Destroy(); err != nil {
				return nil, err
			}

			if idx == len(m.sessions)-1 {
				m.sessions = m.sessions[:idx]
			} else {
				m.sessions = append(m.sessions[:idx], m.sessions[idx+1:]...)
			}

			return &kernelv1.DeleteSessionResponse{}, nil
		}
	}
	return nil, errors.New("session does not exist")
}

func (m *kernelServiceServer) Execute(req *kernelv1.ExecuteRequest, srv kernelv1.KernelService_ExecuteServer) error {
	sess := m.findSession(req.SessionId)
	if sess == nil {
		return errors.New("session not found")
	}

	buf := new(bytes.Buffer)
	execDone := make(chan struct{})
	g, _ := errgroup.WithContext(srv.Context())

	g.Go(func() error {
		for {
			p := make([]byte, 1024)
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

		exitCode, err := sess.Execute(command, buf)
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

func (m *kernelServiceServer) findSession(id string) *Session {
	for _, session := range m.sessions {
		if session.ID() == id {
			return session
		}
	}
	return nil
}

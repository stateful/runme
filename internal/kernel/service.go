package kernel

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	kernelv1 "github.com/stateful/runme/internal/gen/proto/go/kernel/v1"
)

type KernelServiceServer struct {
	kernelv1.UnimplementedKernelServiceServer

	sessions []*Session
}

func (m *KernelServiceServer) PostSession(ctx context.Context, req *kernelv1.PostSessionRequest) (*kernelv1.PostSessionResponse, error) {
	prompt, err := DetectPrompt(req.CommandName)
	if err != nil {
		return nil, fmt.Errorf("failed to detect prompt: %w", err)
	}

	s, err := NewSession(prompt, req.CommandName)
	if err != nil {
		return nil, err
	}

	m.sessions = append(m.sessions, s)

	return &kernelv1.PostSessionResponse{
		Session: &kernelv1.Session{Id: s.id},
	}, nil
}

func (m *KernelServiceServer) DeleteSession(ctx context.Context, req *kernelv1.DeleteSessionRequest) (*kernelv1.DeleteSessionResponse, error) {
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

func (m *KernelServiceServer) Execute(req *kernelv1.ExecuteRequest, srv kernelv1.KernelService_ExecuteServer) error {
	sess := m.findSession(req.SessionId)
	if sess == nil {
		return errors.New("session not found")
	}

	errC := make(chan error, 1)
	done := make(chan struct{}) // finished executing the command
	buf := bytes.NewBuffer(nil)

	go func() {
		for {
			select {
			case <-time.After(time.Second):
			case <-done:
				return
			}
			err := srv.Send(&kernelv1.ExecuteResponse{
				Stdout: buf.Bytes(),
			})
			if err != nil {
				errC <- err
				return
			}
		}
	}()

	go func() {
		exitCode, err := sess.Execute(req.Command, buf)
		close(done)
		if err != nil {
			errC <- err
		} else {
			errC <- srv.Send(&kernelv1.ExecuteResponse{
				ExitCode: uint32(exitCode),
			})
		}
	}()

	return <-errC
}

func (m *KernelServiceServer) findSession(id string) *Session {
	for _, session := range m.sessions {
		if session.ID() == id {
			return session
		}
	}
	return nil
}

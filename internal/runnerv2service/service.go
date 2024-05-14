package runnerv2service

import (
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stateful/runme/v3/internal/command"
	runnerv2alpha1 "github.com/stateful/runme/v3/pkg/gen/proto/go/runme/runner/v2alpha1"
)

type runnerService struct {
	runnerv2alpha1.UnimplementedRunnerServiceServer

	sessions *command.SessionList
	logger   *zap.Logger
}

func NewRunnerService(logger *zap.Logger) (runnerv2alpha1.RunnerServiceServer, error) {
	sessions, err := command.NewSessionList()
	if err != nil {
		return nil, err
	}
	return &runnerService{
		sessions: sessions,
		logger:   logger,
	}, nil
}

type requestWithSession interface {
	GetSessionId() string
	GetSessionStrategy() runnerv2alpha1.SessionStrategy
}

func (r *runnerService) getSessionFromRequest(req requestWithSession) (*command.Session, bool, error) {
	var (
		session *command.Session
		found   bool
	)

	switch req.GetSessionStrategy() {
	case runnerv2alpha1.SessionStrategy_SESSION_STRATEGY_UNSPECIFIED:
		if req.GetSessionId() != "" {
			session, found = r.sessions.Get(req.GetSessionId())
			if !found {
				return nil, false, status.Errorf(codes.NotFound, "session %q not found", req.GetSessionId())
			}
		}
	case runnerv2alpha1.SessionStrategy_SESSION_STRATEGY_MOST_RECENT:
		session, found = r.sessions.Newest()
	}

	return session, found, nil
}

func (r *runnerService) getOrCreateSessionFromRequest(req requestWithSession) (_ *command.Session, exists bool, _ error) {
	var (
		session *command.Session
		found   bool
	)

	switch req.GetSessionStrategy() {
	case runnerv2alpha1.SessionStrategy_SESSION_STRATEGY_UNSPECIFIED:
		if req.GetSessionId() != "" {
			session, found = r.sessions.Get(req.GetSessionId())
			if !found {
				return nil, false, status.Errorf(codes.NotFound, "session %q not found", req.GetSessionId())
			}
		} else {
			session = command.NewSession()
		}
	case runnerv2alpha1.SessionStrategy_SESSION_STRATEGY_MOST_RECENT:
		session, found = r.sessions.Newest()
		if !found {
			session = command.NewSession()
		}
	}

	return session, found, nil
}

package runnerv2service

import (
	"os"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stateful/runme/v3/internal/command"
	"github.com/stateful/runme/v3/internal/lru"
	"github.com/stateful/runme/v3/internal/session"
	runnerv2 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/runner/v2"
	"github.com/stateful/runme/v3/pkg/project"
)

type runnerService struct {
	runnerv2.UnimplementedRunnerServiceServer

	cmdFactory command.Factory
	sessions   *lru.Cache[*session.Session]
	logger     *zap.Logger
}

func NewRunnerService(factory command.Factory, logger *zap.Logger) (runnerv2.RunnerServiceServer, error) {
	sessions := session.NewSessionList()

	r := &runnerService{
		cmdFactory: factory,
		sessions:   sessions,
		logger:     logger,
	}

	return r, nil
}

type requestWithSession interface {
	GetSessionId() string
	GetSessionStrategy() runnerv2.SessionStrategy
}

func (r *runnerService) getSessionFromRequest(req requestWithSession) (*session.Session, bool, error) {
	var (
		session *session.Session
		found   bool
	)

	switch req.GetSessionStrategy() {
	case runnerv2.SessionStrategy_SESSION_STRATEGY_UNSPECIFIED:
		if req.GetSessionId() != "" {
			session, found = r.sessions.GetByID(req.GetSessionId())
			if !found {
				return nil, false, status.Errorf(codes.NotFound, "session %q not found", req.GetSessionId())
			}
		}
	case runnerv2.SessionStrategy_SESSION_STRATEGY_MOST_RECENT:
		session, found = r.sessions.Newest()
	}

	return session, found, nil
}

func (r *runnerService) getOrCreateSessionFromRequest(req requestWithSession, proj *project.Project) (_ *session.Session, exists bool, _ error) {
	var (
		sess  *session.Session
		found bool
	)

	// TODO(adamb): this should come from the runme.yaml in the future.
	seedEnv := os.Environ()

	switch req.GetSessionStrategy() {
	case runnerv2.SessionStrategy_SESSION_STRATEGY_UNSPECIFIED:
		if req.GetSessionId() != "" {
			sess, found = r.sessions.GetByID(req.GetSessionId())
			if !found {
				return nil, false, status.Errorf(codes.NotFound, "session %q not found", req.GetSessionId())
			}
		} else {
			s, err := session.New(
				session.WithOwl(false),
				session.WithProject(proj),
				session.WithSeedEnv(seedEnv),
			)
			if err != nil {
				return nil, false, status.Errorf(codes.Internal, "failed to create new session: %v", err)
			}
			sess = s
		}
	case runnerv2.SessionStrategy_SESSION_STRATEGY_MOST_RECENT:
		sess, found = r.sessions.Newest()
		if !found {
			s, err := session.New(
				session.WithOwl(false),
				session.WithProject(proj),
				session.WithSeedEnv(seedEnv),
			)
			if err != nil {
				return nil, false, status.Errorf(codes.Internal, "failed to create new session: %v", err)
			}
			sess = s
		}
	}

	return sess, found, nil
}

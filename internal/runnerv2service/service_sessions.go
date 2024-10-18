package runnerv2service

import (
	"context"
	"os"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stateful/runme/v3/internal/command"
	rcontext "github.com/stateful/runme/v3/internal/runner/context"
	runnerv2 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/runner/v2"
	"github.com/stateful/runme/v3/pkg/project"
)

func convertSessionToRunnerv2alpha1Session(sess *command.Session) *runnerv2.Session {
	return &runnerv2.Session{
		Id:  sess.ID,
		Env: sess.GetAllEnv(),
		// Metadata: sess.Metadata,
	}
}

// TODO(adamb): this function should not return nil project and nil error at the same time.
func convertProtoProjectToProject(runnerProj *runnerv2.Project) (*project.Project, error) {
	if runnerProj == nil {
		return nil, nil
	}

	opts := project.DefaultProjectOptions[:]

	if runnerProj.EnvLoadOrder != nil {
		opts = append(opts, project.WithEnvFilesReadOrder(runnerProj.EnvLoadOrder))
	}

	return project.NewDirProject(runnerProj.Root, opts...)
}

func (r *runnerService) CreateSession(ctx context.Context, req *runnerv2.CreateSessionRequest) (*runnerv2.CreateSessionResponse, error) {
	r.logger.Info("running CreateSession in runnerService")

	proj, err := convertProtoProjectToProject(req.GetProject())
	if err != nil {
		return nil, err
	}

	owl := false
	var seedEnv []string

	cfg := req.GetConfig()
	if cfg != nil {
		if cfg.GetEnvStoreSeeding() == runnerv2.CreateSessionRequest_Config_SESSION_ENV_STORE_SEEDING_SYSTEM {
			seedEnv = os.Environ()
		}
		if cfg.GetEnvStoreType() == runnerv2.SessionEnvStoreType_SESSION_ENV_STORE_TYPE_OWL {
			owl = true
		}
	}

	sess, err := command.NewSession(command.WithOwl(owl), command.WithSessionProject(proj), command.WithSeedEnv(seedEnv))
	if err != nil {
		return nil, err
	}

	if err := r.updateSession(ctx, sess, req); err != nil {
		return nil, err
	}

	r.sessions.Add(sess)

	r.logger.Debug("created session", zap.String("id", sess.ID), zap.Bool("owl", owl), zap.Int("seed_env_len", len(seedEnv)))

	return &runnerv2.CreateSessionResponse{
		Session: convertSessionToRunnerv2alpha1Session(sess),
	}, nil
}

func (r *runnerService) GetSession(_ context.Context, req *runnerv2.GetSessionRequest) (*runnerv2.GetSessionResponse, error) {
	r.logger.Info("running GetSession in runnerService")

	sess, ok := r.sessions.Get(req.Id)
	if !ok {
		return nil, status.Error(codes.NotFound, "session not found")
	}

	return &runnerv2.GetSessionResponse{
		Session: convertSessionToRunnerv2alpha1Session(sess),
	}, nil
}

func (r *runnerService) ListSessions(_ context.Context, req *runnerv2.ListSessionsRequest) (*runnerv2.ListSessionsResponse, error) {
	r.logger.Info("running ListSessions in runnerService")

	sessions := r.sessions.List()

	runnerSessions := make([]*runnerv2.Session, 0, len(sessions))
	for _, s := range sessions {
		runnerSessions = append(runnerSessions, convertSessionToRunnerv2alpha1Session(s))
	}

	return &runnerv2.ListSessionsResponse{Sessions: runnerSessions}, nil
}

func (r *runnerService) UpdateSession(ctx context.Context, req *runnerv2.UpdateSessionRequest) (*runnerv2.UpdateSessionResponse, error) {
	r.logger.Info("running UpdateSession in runnerService")

	sess, ok := r.sessions.Get(req.Id)
	if !ok {
		return nil, status.Error(codes.NotFound, "session not found")
	}

	if err := r.updateSession(ctx, sess, req); err != nil {
		return nil, err
	}

	return &runnerv2.UpdateSessionResponse{Session: convertSessionToRunnerv2alpha1Session(sess)}, nil
}

func (r *runnerService) DeleteSession(_ context.Context, req *runnerv2.DeleteSessionRequest) (*runnerv2.DeleteSessionResponse, error) {
	r.logger.Info("running DeleteSession in runnerService")

	deleted := r.sessions.Delete(req.Id)

	if !deleted {
		return nil, status.Error(codes.NotFound, "session not found")
	}

	return &runnerv2.DeleteSessionResponse{}, nil
}

type updateRequest interface {
	GetEnv() []string
	GetProject() *runnerv2.Project
}

func (r *runnerService) updateSession(ctx context.Context, sess *command.Session, req updateRequest) error {
	ctx = rcontext.ContextWithExecutionInfo(ctx, &rcontext.ExecutionInfo{
		ExecContext: "request",
	})

	return sess.SetEnv(ctx, req.GetEnv()...)
}

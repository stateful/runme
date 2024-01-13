package runnerv2service

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	runnerv2alpha1 "github.com/stateful/runme/internal/gen/proto/go/runme/runner/v2alpha1"
	"github.com/stateful/runme/internal/project"
	"github.com/stateful/runme/internal/runner"
)

func toRunnerv2alpha1Session(sess *runner.Session) *runnerv2alpha1.Session {
	return &runnerv2alpha1.Session{
		Id:       sess.ID,
		Env:      sess.Envs(),
		Metadata: sess.Metadata,
	}
}

func convertProtoProjectToProject(runnerProj *runnerv2alpha1.Project) (*project.Project, error) {
	if runnerProj == nil {
		return nil, nil
	}

	opts := []project.ProjectOption{
		project.WithFindRepoUpward(),
		project.WithEnvFilesReadOrder(runnerProj.EnvLoadOrder),
	}

	return project.NewDirProject(runnerProj.Root, opts...)
}

func (r *runnerService) CreateSession(ctx context.Context, req *runnerv2alpha1.CreateSessionRequest) (*runnerv2alpha1.CreateSessionResponse, error) {
	r.logger.Info("running CreateSession in runnerService")

	proj, err := convertProtoProjectToProject(req.Project)
	if err != nil {
		return nil, err
	}

	env := make([]string, len(req.Env))
	copy(env, req.Env)

	if proj != nil {
		projEnvs, err := proj.LoadEnvs()
		if err != nil {
			return nil, err
		}

		env = append(env, projEnvs...)
	}

	sess, err := runner.NewSession(env, r.logger)
	if err != nil {
		return nil, err
	}

	r.sessions.AddSession(sess)

	return &runnerv2alpha1.CreateSessionResponse{
		Session: toRunnerv2alpha1Session(sess),
	}, nil
}

func (r *runnerService) GetSession(_ context.Context, req *runnerv2alpha1.GetSessionRequest) (*runnerv2alpha1.GetSessionResponse, error) {
	r.logger.Info("running GetSession in runnerService")

	sess, ok := r.sessions.GetSession(req.Id)

	if !ok {
		return nil, status.Error(codes.NotFound, "session not found")
	}

	return &runnerv2alpha1.GetSessionResponse{
		Session: toRunnerv2alpha1Session(sess),
	}, nil
}

func (r *runnerService) ListSessions(_ context.Context, req *runnerv2alpha1.ListSessionsRequest) (*runnerv2alpha1.ListSessionsResponse, error) {
	r.logger.Info("running ListSessions in runnerService")

	sessions, err := r.sessions.ListSessions()
	if err != nil {
		return nil, err
	}

	runnerSessions := make([]*runnerv2alpha1.Session, 0, len(sessions))
	for _, s := range sessions {
		runnerSessions = append(runnerSessions, toRunnerv2alpha1Session(s))
	}

	return &runnerv2alpha1.ListSessionsResponse{Sessions: runnerSessions}, nil
}

func (r *runnerService) DeleteSession(_ context.Context, req *runnerv2alpha1.DeleteSessionRequest) (*runnerv2alpha1.DeleteSessionResponse, error) {
	r.logger.Info("running DeleteSession in runnerService")

	deleted := r.sessions.DeleteSession(req.Id)

	if !deleted {
		return nil, status.Error(codes.NotFound, "session not found")
	}
	return &runnerv2alpha1.DeleteSessionResponse{}, nil
}

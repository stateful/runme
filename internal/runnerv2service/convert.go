package runnerv2service

import (
	"github.com/stateful/runme/v3/internal/session"
	runnerv2 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/runner/v2"
	"github.com/stateful/runme/v3/pkg/project"
)

func convertSessionToProtoSession(sess *session.Session) *runnerv2.Session {
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

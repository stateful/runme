package projectservice

import (
	"errors"

	projectv1 "github.com/stateful/runme/internal/gen/proto/go/runme/project/v1"
	"github.com/stateful/runme/internal/project"
	"go.uber.org/zap"
)

type projectServiceServer struct {
	projectv1.UnimplementedProjectServiceServer

	logger *zap.Logger
}

func NewProjectServiceServer(logger *zap.Logger) projectv1.ProjectServiceServer {
	return &projectServiceServer{logger: logger}
}

func (s *projectServiceServer) Load(req *projectv1.LoadRequest, srv projectv1.ProjectService_LoadServer) error {
	_, err := projectFromReq(req)
	return err
}

func projectFromReq(req *projectv1.LoadRequest) (*project.Project, error) {
	switch v := req.GetKind().(type) {
	case *projectv1.LoadRequest_Directory:
		var opts []project.ProjectOption

		if v.Directory.RespectGitignore {
			opts = append(opts, project.WithRespectGitignore())
		}

		if patterns := v.Directory.IgnoreFilePatterns; len(patterns) > 0 {
			opts = append(opts, project.WithIgnoreFilePatterns(patterns...))
		}

		if v.Directory.FindRepoUpward {
			opts = append(opts, project.WithFindRepoUpward())
		}

		return project.NewFileProject(v.Directory.Path, opts...)
	case *projectv1.LoadRequest_File:
		return project.NewFileProject(v.File.Path)
	default:
		return nil, errors.New("unknown request kind")
	}
}

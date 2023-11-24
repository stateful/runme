package projectservice

import (
	"context"
	"errors"
	"sync"

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
	proj, err := projectFromReq(req)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(srv.Context())
	eventc := make(chan project.LoadEvent)
	errc := make(chan error, 1)
	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(errc)
		for event := range eventc {
			msg := &projectv1.LoadResponse{
				Type: projectv1.LoadEventType(event.Type),
			}

			if err := setDataForLoadResponseFromLoadEvent(msg, event); err != nil {
				errc <- err
				goto errhandler
			}

			if err := srv.Send(msg); err != nil {
				errc <- err
				goto errhandler
			}

			continue

		errhandler:
			cancel()
			// Project.Load() should be notified that it should exit early
			// via cancel(). eventc will be closed, but it should be drained too
			// in order to clean up any in-flight events.
			// In theory, this is not necessary provided that all sends to eventc
			// are wrapped in selects, which observe ctx.Done().
			//revive:disable:empty-block
			for range eventc {
			}
			//revive:enable:empty-block
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		proj.Load(ctx, eventc, false)
	}()

	wg.Wait()

	return <-errc
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

		return project.NewDirProject(v.Directory.Path, opts...)
	case *projectv1.LoadRequest_File:
		return project.NewFileProject(v.File.Path)
	default:
		return nil, errors.New("unknown request kind")
	}
}

func setDataForLoadResponseFromLoadEvent(resp *projectv1.LoadResponse, event project.LoadEvent) error {
	switch event.Type {
	case project.LoadEventStartedWalk:
	case project.LoadEventFoundDir:
		var data project.LoadEventFoundDirData
		event.ExtractDataValue(&data)

		resp.Data = &projectv1.LoadResponse_FoundDir{
			FoundDir: &projectv1.LoadEventFoundDir{
				Dir: data.Dir,
			},
		}
	case project.LoadEventFoundFile:
	case project.LoadEventFinishedWalk:
	case project.LoadEventStartedParsingDocument:
	case project.LoadEventFinishedParsingDocument:
	case project.LoadEventFoundTask:
	case project.LoadEventError:
	default:
		return errors.New("unknown LoadEventType while converting project.LoadEvent to projectv1.LoadResponse")
	}

	return nil
}

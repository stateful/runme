package projectservice

import (
	"context"
	"errors"
	"sync"

	"go.uber.org/zap"

	projectv1 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/project/v1"
	"github.com/stateful/runme/v3/pkg/project"
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
			// are wrapped in selects which observe ctx.Done().
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
		opts := []project.ProjectOption{
			project.WithRespectGitignore(!v.Directory.SkipGitignore),
			project.WithIgnoreFilePatterns(v.Directory.IgnoreFilePatterns...),
		}

		if !v.Directory.SkipRepoLookupUpward {
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
		resp.Data = &projectv1.LoadResponse_StartedWalk{
			StartedWalk: &projectv1.LoadEventStartedWalk{},
		}
	case project.LoadEventFoundDir:
		data := project.ExtractDataFromLoadEvent[project.LoadEventFoundDirData](event)

		resp.Data = &projectv1.LoadResponse_FoundDir{
			FoundDir: &projectv1.LoadEventFoundDir{
				Path: data.Path,
			},
		}
	case project.LoadEventFoundFile:
		data := project.ExtractDataFromLoadEvent[project.LoadEventFoundFileData](event)

		resp.Data = &projectv1.LoadResponse_FoundFile{
			FoundFile: &projectv1.LoadEventFoundFile{
				Path: data.Path,
			},
		}
	case project.LoadEventFinishedWalk:
		resp.Data = &projectv1.LoadResponse_FinishedWalk{
			FinishedWalk: &projectv1.LoadEventFinishedWalk{},
		}
	case project.LoadEventStartedParsingDocument:
		data := project.ExtractDataFromLoadEvent[project.LoadEventStartedParsingDocumentData](event)

		resp.Data = &projectv1.LoadResponse_StartedParsingDoc{
			StartedParsingDoc: &projectv1.LoadEventStartedParsingDoc{
				Path: data.Path,
			},
		}
	case project.LoadEventFinishedParsingDocument:
		data := project.ExtractDataFromLoadEvent[project.LoadEventFinishedParsingDocumentData](event)

		resp.Data = &projectv1.LoadResponse_FinishedParsingDoc{
			FinishedParsingDoc: &projectv1.LoadEventFinishedParsingDoc{
				Path: data.Path,
			},
		}
	case project.LoadEventFoundTask:
		data := project.ExtractDataFromLoadEvent[project.LoadEventFoundTaskData](event)

		resp.Data = &projectv1.LoadResponse_FoundTask{
			FoundTask: &projectv1.LoadEventFoundTask{
				DocumentPath:    data.Task.DocumentPath,
				Id:              data.Task.CodeBlock.ID(),
				Name:            data.Task.CodeBlock.Name(),
				IsNameGenerated: data.Task.CodeBlock.IsUnnamed(),
			},
		}
	case project.LoadEventError:
		data := project.ExtractDataFromLoadEvent[project.LoadEventErrorData](event)

		resp.Data = &projectv1.LoadResponse_Error{
			Error: &projectv1.LoadEventError{
				ErrorMessage: data.Err.Error(),
			},
		}
	default:
		return errors.New("unknown LoadEventType while converting project.LoadEvent to projectv1.LoadResponse")
	}

	return nil
}

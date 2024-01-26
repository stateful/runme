package runnerv2service

import (
	"context"
	"slices"
	"strings"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stateful/runme/internal/command"
	runnerv2alpha1 "github.com/stateful/runme/internal/gen/proto/go/runme/runner/v2alpha1"
)

func (r *runnerService) ResolveEnv(ctx context.Context, req *runnerv2alpha1.ResolveEnvRequest) (*runnerv2alpha1.ResolveEnvResponse, error) {
	// Add explicitly passed env as a source.
	sources := []command.EnvResolverSource{
		command.EnvResolverSourceFunc(req.Env),
	}

	// Add project env as a source.
	proj, err := convertProtoProjectToProject(req.GetProject())
	if err != nil {
		return nil, err
	}
	if proj != nil {
		projEnvs, err := proj.LoadEnv()
		if err != nil {
			r.logger.Info("failed to load envs for project", zap.Error(err))
		} else {
			sources = append(sources, command.EnvResolverSourceFunc(projEnvs))
		}
	}

	// Add session env as a source.
	session, found, err := r.getSessionFromRequest(req)
	if err != nil {
		return nil, err
	}
	if found {
		sources = append(sources, command.EnvResolverSourceFunc(session.GetEnv()))
	}

	resolver := command.NewEnvResolver(sources...)

	var result []*runnerv2alpha1.ResolveEnvResult

	if script := req.GetScript(); script != "" {
		result, err = resolver.Resolve(strings.NewReader(script))
	} else if commands := req.GetCommands(); commands != nil && len(commands.Items) > 0 {
		result, err = resolver.Resolve(strings.NewReader(strings.Join(commands.Items, "\n")))
	} else {
		err = status.Error(codes.InvalidArgument, "either script or commands must be provided")
	}
	if err != nil {
		return nil, err
	}

	slices.SortStableFunc(result, func(a, b *runnerv2alpha1.ResolveEnvResult) int {
		aResolved, bResolved := a.GetResolvedEnv(), b.GetResolvedEnv()
		if aResolved != nil && bResolved != nil {
			return strings.Compare(aResolved.Name, bResolved.Name)
		}
		if aResolved != nil {
			return -1
		}
		return 1
	})

	return &runnerv2alpha1.ResolveEnvResponse{Items: result}, nil
}

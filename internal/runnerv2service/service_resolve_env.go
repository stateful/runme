package runnerv2service

import (
	"context"
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

	var result []*command.EnvResolverResult

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

	response := &runnerv2alpha1.ResolveEnvResponse{}

	for _, item := range result {
		response.Items = append(response.Items, &runnerv2alpha1.ResolveEnvResult{
			Name:          item.Name,
			OriginalValue: item.OriginalValue,
			ResolvedValue: item.Value,
		})
	}

	return response, nil
}

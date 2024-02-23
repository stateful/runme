package runnerv2service

import (
	"bytes"
	"context"
	"strings"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stateful/runme/internal/command"
	runnerv2alpha1 "github.com/stateful/runme/internal/gen/proto/go/runme/runner/v2alpha1"
)

func (r *runnerService) ResolveVars(ctx context.Context, req *runnerv2alpha1.ResolveVarsRequest) (*runnerv2alpha1.ResolveVarsResponse, error) {
	// Add explicitly passed env as a source.
	sources := []command.ProgramResolverSource{
		command.ProgramResolverSourceFunc(req.Env),
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
			sources = append(sources, command.ProgramResolverSourceFunc(projEnvs))
		}
	}

	// Add session env as a source.
	session, found, err := r.getSessionFromRequest(req)
	if err != nil {
		return nil, err
	}
	if found {
		sources = append(sources, command.ProgramResolverSourceFunc(session.GetEnv()))
	}

	resolver := command.NewProgramResolver(command.ProgramResolverModeAuto, sources...)

	var varRes []*command.ProgramResolverResult
	var scriptRes bytes.Buffer

	if script := req.GetScript(); script != "" {
		varRes, err = resolver.Resolve(strings.NewReader(script), &scriptRes)
	} else if commands := req.GetCommands(); commands != nil && len(commands.Items) > 0 {
		varRes, err = resolver.Resolve(strings.NewReader(strings.Join(commands.Items, "\n")), &scriptRes)
	} else {
		err = status.Error(codes.InvalidArgument, "either script or commands must be provided")
	}
	if err != nil {
		return nil, err
	}

	response := &runnerv2alpha1.ResolveVarsResponse{}

	for _, item := range varRes {
		response.Items = append(response.Items, &runnerv2alpha1.ResolveVarsResult{
			Name:          item.Name,
			OriginalValue: item.OriginalValue,
			ResolvedValue: item.Value,
		})
	}

	return response, nil
}

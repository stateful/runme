package runnerv2service

import (
	"bytes"
	"context"
	"strings"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stateful/runme/v3/internal/command"
	runnerv2 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/runner/v2"
)

func (r *runnerService) ResolveProgram(ctx context.Context, req *runnerv2.ResolveProgramRequest) (*runnerv2.ResolveProgramResponse, error) {
	r.logger.Info("running ResolveProgram in runnerService")

	resolver, err := r.getProgramResolverFromReq(req)
	if err != nil {
		return nil, err
	}

	var (
		result            *command.ProgramResolverResult
		modifiedScriptBuf bytes.Buffer
	)

	if script := req.GetScript(); script != "" {
		result, err = resolver.Resolve(strings.NewReader(script), &modifiedScriptBuf)
	} else if commands := req.GetCommands(); commands != nil && len(commands.Lines) > 0 {
		script := strings.Join(commands.Lines, "\n")
		result, err = resolver.Resolve(strings.NewReader(script), &modifiedScriptBuf)
	} else {
		err = status.Error(codes.InvalidArgument, "either script or commands must be provided")
	}
	if err != nil {
		return nil, err
	}

	modifiedScript := modifiedScriptBuf.String()

	// todo(sebastian): figure out how to return commands
	response := &runnerv2.ResolveProgramResponse{
		Script: modifiedScript,
	}

	for _, item := range result.Variables {
		ritem := &runnerv2.ResolveProgramResponse_VarResult{
			Name:          item.Name,
			OriginalValue: item.OriginalValue,
			ResolvedValue: item.Value,
		}

		switch item.Status {
		case command.ProgramResolverStatusResolved:
			ritem.Status = runnerv2.ResolveProgramResponse_STATUS_RESOLVED
		case command.ProgramResolverStatusUnresolvedWithMessage:
			ritem.Status = runnerv2.ResolveProgramResponse_STATUS_UNRESOLVED_WITH_MESSAGE
		case command.ProgramResolverStatusUnresolvedWithPlaceholder:
			ritem.Status = runnerv2.ResolveProgramResponse_STATUS_UNRESOLVED_WITH_PLACEHOLDER
		case command.ProgramResolverStatusUnresolvedWithSecret:
			ritem.Status = runnerv2.ResolveProgramResponse_STATUS_UNRESOLVED_WITH_SECRET
		default:
			ritem.Status = runnerv2.ResolveProgramResponse_STATUS_UNSPECIFIED
		}

		response.Vars = append(response.Vars, ritem)
	}

	return response, nil
}

func (r *runnerService) getProgramResolverFromReq(req *runnerv2.ResolveProgramRequest) (*command.ProgramResolver, error) {
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

	// todo(sebastian): bring back sensitive keys for owl store
	// Add session env as a source and pass info about sensitive env vars.
	sensitiveEnvKeys := []string{}
	session, found, _ := r.getSessionFromRequest(req)
	if found {
		env := session.GetAllEnv()
		sources = append(sources, command.ProgramResolverSourceFunc(env))

		// sensitiveEnvKeys, err = session.SensitiveEnvKeys()
		// if err != nil {
		// 	return nil, err
		// }
	}

	mode := command.ProgramResolverModeAuto

	switch req.GetMode() {
	case runnerv2.ResolveProgramRequest_MODE_PROMPT_ALL:
		mode = command.ProgramResolverModePromptAll
	case runnerv2.ResolveProgramRequest_MODE_SKIP_ALL:
		mode = command.ProgramResolverModeSkipAll
	}

	return command.NewProgramResolver(mode, sensitiveEnvKeys, sources...), err
}

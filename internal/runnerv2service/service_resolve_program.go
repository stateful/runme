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

func (r *runnerService) ResolveProgram(ctx context.Context, req *runnerv2alpha1.ResolveProgramRequest) (*runnerv2alpha1.ResolveProgramResponse, error) {
	r.logger.Info("running ResolveProgram in runnerService")

	resolver, err := r.getProgramResolverFromReq(req)
	if err != nil {
		return nil, err
	}

	var (
		result         *command.ProgramResolverResult
		modifiedScript bytes.Buffer
	)

	if script := req.GetScript(); script != "" {
		result, err = resolver.Resolve(strings.NewReader(script), &modifiedScript)
	} else if commands := req.GetCommands(); commands != nil && len(commands.Lines) > 0 {
		result, err = resolver.Resolve(strings.NewReader(strings.Join(commands.Lines, "\n")), &modifiedScript)
	} else {
		err = status.Error(codes.InvalidArgument, "either script or commands must be provided")
	}
	if err != nil {
		return nil, err
	}

	response := &runnerv2alpha1.ResolveProgramResponse{
		Commands: &runnerv2alpha1.ResolveProgramCommandList{
			Lines: strings.Split(modifiedScript.String(), "\n"),
		},
	}

	for _, item := range result.Variables {
		ritem := &runnerv2alpha1.ResolveProgramResponse_VarsResult{
			Name:          item.Name,
			OriginalValue: item.OriginalValue,
			ResolvedValue: item.Value,
		}

		switch item.Status {
		case command.ProgramResolverStatusResolved:
			ritem.Status = runnerv2alpha1.ResolveProgramResponse_VARS_PROMPT_RESOLVED
		case command.ProgramResolverStatusUnresolvedWithMessage:
			ritem.Status = runnerv2alpha1.ResolveProgramResponse_VARS_PROMPT_MESSAGE
		case command.ProgramResolverStatusUnresolvedWithPlaceholder:
			ritem.Status = runnerv2alpha1.ResolveProgramResponse_VARS_PROMPT_PLACEHOLDER
		default:
			ritem.Status = runnerv2alpha1.ResolveProgramResponse_VARS_PROMPT_UNSPECIFIED
		}

		response.Vars = append(response.Vars, ritem)
	}

	return response, nil
}

func (r *runnerService) getProgramResolverFromReq(req *runnerv2alpha1.ResolveProgramRequest) (*command.ProgramResolver, error) {
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

	// Add session env as a source. If session is not found, it's not an error.
	session, found, _ := r.getSessionFromRequest(req)
	if found {
		sources = append(sources, command.ProgramResolverSourceFunc(session.GetEnv()))
	}

	mode := command.ProgramResolverModeAuto

	switch req.GetVarsMode() {

	case runnerv2alpha1.ResolveProgramRequest_VARS_MODE_PROMPT:
		mode = command.ProgramResolverModePrompt
	case runnerv2alpha1.ResolveProgramRequest_VARS_MODE_SKIP:
		mode = command.ProgramResolverModeSkip
	}

	return command.NewProgramResolver(mode, sources...), err
}

package client

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/muesli/cancelreader"
	"github.com/pkg/errors"
	runnerv1 "github.com/stateful/runme/v3/internal/gen/proto/go/runme/runner/v1"
	"github.com/stateful/runme/v3/internal/project"
	"github.com/stateful/runme/v3/internal/runner"
	"golang.org/x/sync/errgroup"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func runTask(rs *RunnerSettings, ctx context.Context, task project.Task) error {
	block := task.CodeBlock
	fmtr, err := task.CodeBlock.Document().Frontmatter()
	if err != nil {
		return err
	}

	if rs.client == nil {
		return errors.New("client not initialized")
	}

	stream, err := rs.client.Execute(ctx)
	if err != nil {
		return err
	}

	tty := block.Interactive()

	customShell := rs.customShell
	if fmtr != nil && fmtr.Shell != "" {
		customShell = fmtr.Shell
	}

	programName, commandMode := runner.GetCellProgram(block.Language(), customShell, block)

	var commandModeGrpc runnerv1.CommandMode

	switch commandMode {
	case runner.CommandModeNone:
		commandModeGrpc = runnerv1.CommandMode_COMMAND_MODE_UNSPECIFIED
	case runner.CommandModeInlineShell:
		commandModeGrpc = runnerv1.CommandMode_COMMAND_MODE_INLINE_SHELL
	case runner.CommandModeTempFile:
		commandModeGrpc = runnerv1.CommandMode_COMMAND_MODE_TEMP_FILE
	}

	req := &runnerv1.ExecuteRequest{
		ProgramName:     programName,
		Directory:       rs.dir,
		Commands:        block.Lines(),
		Tty:             tty,
		SessionId:       rs.sessionID,
		SessionStrategy: rs.sessionStrategy,
		Background:      block.Background(),
		StoreLastOutput: true,
		Envs:            rs.envs,
		CommandMode:     commandModeGrpc,
		LanguageId:      block.Language(),
	}

	req.Project = ConvertToRunnerProject(rs.project)

	req.Directory = ResolveDirectory(req.Directory, task)

	if rs.sessionStrategy == runnerv1.SessionStrategy_SESSION_STRATEGY_MOST_RECENT {
		req.Envs = os.Environ()
	}

	err = stream.Send(req)
	if err != nil {
		return errors.Wrap(err, "failed to send initial request")
	}

	background := block.Background()
	if !rs.enableBackground {
		background = false
	}

	g := new(errgroup.Group)

	if tty {
		g.Go(func() error { return sendLoop(rs, stream) })
	}

	g.Go(func() error {
		defer func() {
			if canceler, ok := rs.stdin.(cancelreader.CancelReader); ok {
				_ = canceler.Cancel()
			}
		}()
		return recvLoop(rs, stream, background, req.LanguageId)
	})

	return g.Wait()
}

func sendLoop(rs *RunnerSettings, stream runnerv1.RunnerService_ExecuteClient) error {
	buf := make([]byte, 32*1024)

	for {
		n, err := rs.stdin.Read(buf)
		if err != nil {
			if errors.Is(err, io.EOF) {
				err = nil
			}
			return nil
			// return errors.Wrap(err, "failed to read from stdin")
		}
		err = stream.Send(&runnerv1.ExecuteRequest{
			InputData: buf[:n],
		})
		if err != nil {
			return errors.Wrap(err, "failed to send input")
		}
	}
}

func recvLoop(rs *RunnerSettings, stream runnerv1.RunnerService_ExecuteClient, background bool, languageID string) error {
	for {
		msg, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) || status.Convert(err).Code() == codes.Canceled {
				err = nil
			}

			st := status.Convert(err)
			for _, detail := range st.Details() {
				if t, ok := detail.(*errdetails.BadRequest); ok {
					for _, violation := range t.GetFieldViolations() {
						if violation.GetField() == "LanguageId" {
							if strings.Contains(violation.GetDescription(), "unable to find program for language") {
								return errors.Wrapf(err, "invalid language %s", languageID)
							}
						}
					}
				}
			}

			return errors.Wrap(err, "stream closed")
		}

		if len(msg.StdoutData) > 0 {
			_, err := rs.stdout.Write(msg.StdoutData)
			if err != nil {
				return errors.Wrap(err, "failed to write stdout")
			}
		}
		if len(msg.StderrData) > 0 {
			_, err := rs.stderr.Write(msg.StderrData)
			if err != nil {
				return errors.Wrap(err, "failed to write stderr")
			}
		}
		if msg.ExitCode != nil {
			if msg.ExitCode.Value > 0 {
				return &runner.ExitError{Code: uint(msg.ExitCode.Value)}
			}
			return nil
		}
		if msg.Pid != nil && background {
			_, _ = rs.stdout.Write([]byte(fmt.Sprintf("Process started on PID %d\n", msg.Pid.Pid)))
			return nil
		}
	}
}

func setupSession(rs *RunnerSettings, ctx context.Context) error {
	if rs.sessionID != "" || rs.sessionStrategy == runnerv1.SessionStrategy_SESSION_STRATEGY_MOST_RECENT {
		return nil
	}

	envs := append(os.Environ(), rs.envs...)

	request := &runnerv1.CreateSessionRequest{
		Envs:         envs,
		Project:      ConvertToRunnerProject(rs.project),
		EnvStoreType: rs.envStoreType,
	}

	resp, err := rs.client.CreateSession(ctx, request)
	if err != nil {
		return errors.Wrap(err, "failed to create session")
	}

	rs.sessionID = resp.Session.Id

	return nil
}

func getEnvs(rs *RunnerSettings, ctx context.Context) ([]string, error) {
	if rs.sessionID == "" {
		return nil, nil
	}

	resp, err := rs.client.GetSession(ctx, &runnerv1.GetSessionRequest{
		Id: rs.sessionID,
	})
	if err != nil {
		return nil, err
	}

	return resp.Session.Envs, nil
}

package client

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/muesli/cancelreader"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	runnerv1 "github.com/stateful/runme/internal/gen/proto/go/runme/runner/v1"
	"github.com/stateful/runme/internal/project"
	"github.com/stateful/runme/internal/runner"
	runmetls "github.com/stateful/runme/internal/tls"
)

type RemoteRunner struct {
	*RunnerSettings
	client runnerv1.RunnerServiceClient
}

func (r *RemoteRunner) Clone() Runner {
	return &RemoteRunner{
		RunnerSettings: r.RunnerSettings.Clone(),
		client:         r.client,
	}
}

func (r *RemoteRunner) getSettings() *RunnerSettings {
	return r.RunnerSettings
}

func (r *RemoteRunner) setSettings(rs *RunnerSettings) {
	r.RunnerSettings = rs
}

func NewRemoteRunner(ctx context.Context, addr string, opts ...RunnerOption) (*RemoteRunner, error) {
	r := &RemoteRunner{
		RunnerSettings: &RunnerSettings{},
	}

	if err := ApplyOptions(r, opts...); err != nil {
		return nil, err
	}

	var creds credentials.TransportCredentials

	if r.insecure {
		creds = insecure.NewCredentials()
	} else {
		tlsConfig, err := runmetls.LoadTLSConfig(r.tlsDir, true)
		if err != nil {
			return nil, err
		}

		creds = credentials.NewTLS(tlsConfig)
	}

	conn, err := grpc.DialContext(ctx, addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to gRPC server")
	}

	r.client = runnerv1.NewRunnerServiceClient(conn)

	if err := r.setupSession(ctx); err != nil {
		return nil, err
	}

	return r, nil
}

func (r *RemoteRunner) setupSession(ctx context.Context) error {
	if r.sessionID != "" || r.sessionStrategy == runnerv1.SessionStrategy_SESSION_STRATEGY_MOST_RECENT {
		return nil
	}

	resp, err := r.client.CreateSession(ctx, &runnerv1.CreateSessionRequest{
		Envs:    os.Environ(),
		Project: ConvertToRunnerProject(r.project),
	})
	if err != nil {
		return errors.Wrap(err, "failed to create session")
	}

	r.sessionID = resp.Session.Id

	return nil
}

func (r *RemoteRunner) deleteSession(ctx context.Context) error {
	if r.sessionID == "" {
		return nil
	}

	_, err := r.client.DeleteSession(ctx, &runnerv1.DeleteSessionRequest{Id: r.sessionID})
	return errors.Wrap(err, "failed to delete session")
}

func ConvertToRunnerProject(proj *project.Project) *runnerv1.Project {
	if proj == nil {
		return nil
	}

	return &runnerv1.Project{
		Root:         proj.Root(),
		EnvLoadOrder: proj.EnvFilesReadOrder(),
	}
}

func (r *RemoteRunner) RunTask(ctx context.Context, task project.Task) error {
	block := task.CodeBlock
	fmtr, err := task.CodeBlock.Document().Frontmatter()
	if err != nil {
		return err
	}

	stream, err := r.client.Execute(ctx)
	if err != nil {
		return err
	}

	tty := block.Interactive()

	customShell := r.customShell
	if fmtr.Shell != "" {
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
		Directory:       r.dir,
		Commands:        block.Lines(),
		Tty:             tty,
		SessionId:       r.sessionID,
		SessionStrategy: r.sessionStrategy,
		Background:      block.Background(),
		StoreLastOutput: true,
		Envs:            r.envs,
		CommandMode:     commandModeGrpc,
		LanguageId:      block.Language(),
	}

	req.Project = ConvertToRunnerProject(r.project)

	req.Directory = ResolveDirectory(req.Directory, task)

	if r.sessionStrategy == runnerv1.SessionStrategy_SESSION_STRATEGY_MOST_RECENT {
		req.Envs = os.Environ()
	}

	err = stream.Send(req)
	if err != nil {
		return errors.Wrap(err, "failed to send initial request")
	}

	background := block.Background()
	if !r.enableBackground {
		background = false
	}

	g := new(errgroup.Group)

	if tty {
		g.Go(func() error { return r.sendLoop(stream, r.stdin) })
	}

	g.Go(func() error {
		defer func() {
			if canceler, ok := r.stdin.(cancelreader.CancelReader); ok {
				_ = canceler.Cancel()
			}
		}()
		return r.recvLoop(stream, background, req.LanguageId)
	})

	return g.Wait()
}

func (r *RemoteRunner) DryRunTask(ctx context.Context, task project.Task, w io.Writer, opts ...RunnerOption) error {
	return ErrRunnerClientUnimplemented
}

func (r *RemoteRunner) Cleanup(ctx context.Context) error {
	if r.cleanupSession {
		if err := r.deleteSession(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (r *RemoteRunner) sendLoop(stream runnerv1.RunnerService_ExecuteClient, stdin io.Reader) error {
	buf := make([]byte, 32*1024)

	for {
		n, err := stdin.Read(buf)
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

func (r *RemoteRunner) recvLoop(stream runnerv1.RunnerService_ExecuteClient, background bool, languageID string) error {
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
			_, err := r.stdout.Write(msg.StdoutData)
			if err != nil {
				return errors.Wrap(err, "failed to write stdout")
			}
		}
		if len(msg.StderrData) > 0 {
			_, err := r.stderr.Write(msg.StderrData)
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
			_, _ = r.stdout.Write([]byte(fmt.Sprintf("Process started on PID %d\n", msg.Pid.Pid)))
			return nil
		}
	}
}

func (r *RemoteRunner) GetEnvs(ctx context.Context) ([]string, error) {
	if r.sessionID == "" {
		return nil, nil
	}

	resp, err := r.client.GetSession(ctx, &runnerv1.GetSessionRequest{
		Id: r.sessionID,
	})
	if err != nil {
		return nil, err
	}

	return resp.Session.Envs, nil
}

package client

import (
	"context"
	"io"

	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	runnerv1 "github.com/stateful/runme/v3/internal/gen/proto/go/runme/runner/v1"
	"github.com/stateful/runme/v3/internal/project"
	runmetls "github.com/stateful/runme/v3/internal/tls"
)

type RemoteRunner struct {
	*RunnerSettings
}

func (r *RemoteRunner) Clone() Runner {
	return &RemoteRunner{
		RunnerSettings: r.RunnerSettings.Clone(),
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
		tlsConfig, err := runmetls.LoadClientConfigFromDir(r.tlsDir)
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

	if err := setupSession(ctx, r.RunnerSettings); err != nil {
		return nil, err
	}

	return r, nil
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
	return runTask(ctx, r.RunnerSettings, task)
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

func (r *RemoteRunner) GetEnvs(ctx context.Context) ([]string, error) {
	return getEnvs(ctx, r.RunnerSettings)
}

func (r *RemoteRunner) GetSessionID() string {
	return r.sessionID
}

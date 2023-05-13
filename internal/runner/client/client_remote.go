package client

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	runnerv1 "github.com/stateful/runme/internal/gen/proto/go/runme/runner/v1"
	"github.com/stateful/runme/internal/project"
	"github.com/stateful/runme/internal/runner"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	runmetls "github.com/stateful/runme/internal/tls"
)

type RemoteRunner struct {
	dir    string
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer

	client          runnerv1.RunnerServiceClient
	sessionID       string
	sessionStrategy runnerv1.SessionStrategy
	cleanupSession  bool

	insecure bool
	tlsDir   string

	enableBackground bool
}

func (r *RemoteRunner) Clone() Runner {
	return &RemoteRunner{
		dir: r.dir,

		stdin:  r.stdin,
		stdout: r.stdout,
		stderr: r.stderr,

		client:          nil,
		sessionID:       r.sessionID,
		sessionStrategy: r.sessionStrategy,
		cleanupSession:  r.cleanupSession,

		insecure: r.insecure,
		tlsDir:   r.tlsDir,

		enableBackground: r.enableBackground,
	}
}

func (r *RemoteRunner) setDir(dir string) error {
	r.dir = dir
	return nil
}

func (r *RemoteRunner) setStdin(stdin io.Reader) error {
	r.stdin = stdin
	return nil
}

func (r *RemoteRunner) setStdout(stdout io.Writer) error {
	r.stdout = stdout
	return nil
}

func (r *RemoteRunner) setStderr(stderr io.Writer) error {
	r.stderr = stderr
	return nil
}

func (r *RemoteRunner) getStdin() io.Reader {
	return r.stdin
}

func (r *RemoteRunner) getStdout() io.Writer {
	return r.stdout
}

func (r *RemoteRunner) getStderr() io.Writer {
	return r.stderr
}

func (r *RemoteRunner) setLogger(logger *zap.Logger) error {
	return nil
}

func (r *RemoteRunner) setSession(session *runner.Session) error {
	r.sessionID = session.ID
	return nil
}

func (r *RemoteRunner) setSessionID(sessionID string) error {
	r.sessionID = sessionID
	return nil
}

func (r *RemoteRunner) setCleanupSession(cleanup bool) error {
	r.cleanupSession = cleanup
	return nil
}

func (r *RemoteRunner) setSessionStrategy(strategy runnerv1.SessionStrategy) error {
	r.sessionStrategy = strategy
	return nil
}

func (r *RemoteRunner) setWithinShell() error {
	return nil
}

func (r *RemoteRunner) setInsecure(insecure bool) error {
	r.insecure = insecure
	return nil
}

func (r *RemoteRunner) setTLSDir(tlsDir string) error {
	r.tlsDir = tlsDir
	return nil
}

func (r *RemoteRunner) setEnableBackgroundProcesses(enableBackground bool) error {
	r.enableBackground = enableBackground
	return nil
}

func NewRemoteRunner(ctx context.Context, addr string, opts ...RunnerOption) (*RemoteRunner, error) {
	r := &RemoteRunner{}

	if err := ApplyOptions(r, opts...); err != nil {
		return nil, err
	}

	var creds credentials.TransportCredentials

	if r.insecure {
		creds = insecure.NewCredentials()
	} else {
		tlsConfig, err := runmetls.LoadTLSConfig(r.tlsDir)
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
		Envs: os.Environ(),
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

func (r *RemoteRunner) RunBlock(ctx context.Context, fileBlock project.FileCodeBlock) error {
	block := fileBlock.GetBlock()

	stream, err := r.client.Execute(ctx)
	if err != nil {
		return err
	}

	tty := block.Interactive()

	req := &runnerv1.ExecuteRequest{
		ProgramName:     runner.ShellPath(),
		Directory:       r.dir,
		Commands:        block.Lines(),
		Tty:             tty,
		SessionId:       r.sessionID,
		SessionStrategy: r.sessionStrategy,
		Background:      block.Background(),
	}

	mdFile := fileBlock.GetFile()
	if mdFile != "" {
		req.Directory = filepath.Join(r.dir, filepath.Dir(mdFile))
	}

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
			if closer, ok := r.stdin.(io.ReadCloser); ok {
				_ = closer.Close()
			}
		}()
		return r.recvLoop(stream, background)
	})

	return g.Wait()
}

func (r *RemoteRunner) DryRunBlock(ctx context.Context, fileBlock project.FileCodeBlock, w io.Writer, opts ...RunnerOption) error {
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

func (r *RemoteRunner) recvLoop(stream runnerv1.RunnerService_ExecuteClient, background bool) error {
	for {
		msg, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) || status.Convert(err).Code() == codes.Canceled {
				err = nil
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

package remote

import (
	"context"
	"io"
	"strconv"

	"github.com/pkg/errors"
	"github.com/stateful/runme/internal/document"
	runnerv1 "github.com/stateful/runme/internal/gen/proto/go/runme/runner/v1"
	"github.com/stateful/runme/internal/runner"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ExitError struct {
	Code uint
}

func (e *ExitError) Error() string {
	return "exit code: " + strconv.Itoa(int(e.Code))
}

type Option func(*Runner) error

func WithDir(dir string) Option {
	return func(r *Runner) error {
		r.dir = dir
		return nil
	}
}

func WithStdin(stdin io.Reader) Option {
	return func(r *Runner) error {
		r.stdin = stdin
		return nil
	}
}

func WithStdout(stdout io.Writer) Option {
	return func(r *Runner) error {
		r.stdout = stdout
		return nil
	}
}

func WithStderr(stderr io.Writer) Option {
	return func(r *Runner) error {
		r.stderr = stderr
		return nil
	}
}

type Runner struct {
	dir    string
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer

	client    runnerv1.RunnerServiceClient
	sessionID string
}

func New(ctx context.Context, addr string, opts ...Option) (*Runner, error) {
	r := &Runner{}

	conn, err := grpc.DialContext(ctx, addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to gRPC server")
	}

	r.client = runnerv1.NewRunnerServiceClient(conn)

	if err := r.setupSession(ctx); err != nil {
		return nil, err
	}

	for _, o := range opts {
		if err := o(r); err != nil {
			return nil, err
		}
	}

	return r, nil
}

func (r *Runner) sendLoop(stream runnerv1.RunnerService_ExecuteClient) error {
	buf := make([]byte, 32*1024)

	for {
		n, err := r.stdin.Read(buf)
		if err != nil {
			if errors.Is(err, io.EOF) {
				err = nil
			}
			return errors.Wrap(err, "failed to read from stdin")
		}
		err = stream.Send(&runnerv1.ExecuteRequest{
			InputData: buf[:n],
		})
		if err != nil {
			return errors.Wrap(err, "failed to send input")
		}
	}
}

func (r *Runner) recvLoop(stream runnerv1.RunnerService_ExecuteClient) error {
	for {
		msg, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
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
				return &ExitError{Code: uint(msg.ExitCode.Value)}
			}
			return nil
		}
	}
}

func (r *Runner) setupSession(ctx context.Context) error {
	resp, err := r.client.CreateSession(ctx, &runnerv1.CreateSessionRequest{})
	if err != nil {
		return errors.Wrap(err, "failed to create session")
	}
	r.sessionID = resp.Session.Id
	return nil
}

func (r *Runner) RunBlock(ctx context.Context, block *document.CodeBlock) error {
	stream, err := r.client.Execute(ctx)
	if err != nil {
		return err
	}

	tty := block.Interactive()

	err = stream.Send(&runnerv1.ExecuteRequest{
		ProgramName: (runner.Shell{}).ProgramPath(),
		Directory:   r.dir,
		Commands:    block.Lines(),
		Tty:         tty,
		SessionId:   r.sessionID,
	})
	if err != nil {
		return errors.Wrap(err, "failed to send initial request")
	}

	g := new(errgroup.Group)

	if tty {
		g.Go(func() error {
			return r.sendLoop(stream)
		})
	}

	g.Go(func() error {
		return r.recvLoop(stream)
	})

	return g.Wait()
}

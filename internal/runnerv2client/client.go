package runnerv2client

import (
	"context"
	"io"
	"reflect"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	runnerv2 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/runner/v2"
)

type Client struct {
	runnerv2.RunnerServiceClient
	conn   *grpc.ClientConn
	logger *zap.Logger
}

func New(target string, logger *zap.Logger, opts ...grpc.DialOption) (*Client, error) {
	client, err := grpc.NewClient(target, opts...)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	serviceClient := &Client{
		RunnerServiceClient: runnerv2.NewRunnerServiceClient(client),
		conn:                client,
		logger:              logger,
	}
	return serviceClient, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

type ExecuteProgramOptions struct {
	SessionID        string
	Stdin            io.ReadCloser
	Stdout           io.Writer
	Stderr           io.Writer
	StoreStdoutInEnv bool
	Winsize          *runnerv2.Winsize
}

func (c *Client) ExecuteProgram(
	ctx context.Context,
	cfg *runnerv2.ProgramConfig,
	opts ExecuteProgramOptions,
) error {
	return c.executeProgram(ctx, cfg, opts)
}

func (c *Client) executeProgram(
	ctx context.Context,
	cfg *runnerv2.ProgramConfig,
	opts ExecuteProgramOptions,
) error {
	stream, err := c.Execute(ctx)
	if err != nil {
		return errors.WithMessage(err, "failed to call Execute()")
	}

	// Send the initial request.
	req := &runnerv2.ExecuteRequest{
		Config:           cfg,
		SessionId:        opts.SessionID,
		StoreStdoutInEnv: opts.StoreStdoutInEnv,
		Winsize:          opts.Winsize,
	}
	if err := stream.Send(req); err != nil {
		return errors.WithMessage(err, "failed to send initial request")
	}

	if stdin := opts.Stdin; !isNil(stdin) {
		go func() {
			defer func() {
				c.logger.Info("finishing reading stdin")
				err := stream.CloseSend()
				if err != nil {
					c.logger.Info("failed to close send", zap.Error(err))
				}
			}()

			c.logger.Info("reading stdin")

			buf := make([]byte, 2*1024*1024)

			for {
				n, err := stdin.Read(buf)
				if err != nil {
					c.logger.Info("failed to read stdin", zap.Error(err))
					break
				}

				c.logger.Info("sending stdin", zap.Int("size", n))

				err = stream.Send(&runnerv2.ExecuteRequest{
					InputData: buf[:n],
				})
				if err != nil {
					c.logger.Info("failed to send stdin", zap.Error(err))
					break
				}
			}
		}()
	}

	for {
		resp, err := stream.Recv()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				c.logger.Info("failed to receive response", zap.Error(err))
			}
			break
		}

		if pid := resp.Pid; pid != nil {
			c.logger.Info("server started a process with PID", zap.Uint32("pid", pid.GetValue()), zap.String("mime", resp.MimeType))
		}

		if stdout := opts.Stdout; !isNil(stdout) {
			_, err = stdout.Write(resp.StdoutData)
			if err != nil {
				return errors.WithMessage(err, "failed to write stdout")
			}
		}

		if stderr := opts.Stderr; !isNil(stderr) {
			_, err = stderr.Write(resp.StderrData)
			if err != nil {
				return errors.WithMessage(err, "failed to write stderr")
			}
		}

		if code := resp.GetExitCode(); code != nil && code.GetValue() != 0 {
			return errors.WithMessagef(err, "exit with code %d", code.GetValue())
		}
	}

	return nil
}

func isNil(val any) bool {
	if val == nil {
		return true
	}

	v := reflect.ValueOf(val)

	switch v.Type().Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Pointer, reflect.UnsafePointer:
		return v.IsNil()
	default:
		return false
	}
}

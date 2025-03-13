package runnerv2client

import (
	"context"
	"io"
	"reflect"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"

	runnerv2 "github.com/runmedev/runme/v3/pkg/api/gen/proto/go/runme/runner/v2"
)

const MaxMsgSize = 32 * 1024 * 1024 // 32 MiB

type Client struct {
	runnerv2.RunnerServiceClient
	grpc_health_v1.HealthClient

	conn   *grpc.ClientConn
	logger *zap.Logger
}

func New(clientConn *grpc.ClientConn, logger *zap.Logger) *Client {
	return &Client{
		RunnerServiceClient: runnerv2.NewRunnerServiceClient(clientConn),
		HealthClient:        grpc_health_v1.NewHealthClient(clientConn),
		logger:              logger.Named("runnerv2client.Client"),
	}
}

func (c *Client) Close() error {
	return c.conn.Close()
}

type ExecuteProgramOptions struct {
	InputData        []byte
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
		InputData:        opts.InputData,
		SessionId:        opts.SessionID,
		StoreStdoutInEnv: opts.StoreStdoutInEnv,
		Winsize:          opts.Winsize,
	}
	if err := stream.Send(req); err != nil {
		return errors.WithMessage(err, "failed to send initial request")
	}

	if stdin := opts.Stdin; !isNil(stdin) {
		// TODO(adamb): reimplement it. There should be a singleton
		// handling and forwarding the stdin. The current implementation
		// does not temrinate multiple stdin readers in the case of
		// running multiple commands using "beta run command1 command2 ... commandN".
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

package runnerv2service

import (
	"io"
	"os"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/stateful/runme/v3/internal/ulid"
	runnerv2 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/runner/v2"
)

func (r *runnerService) Execute(srv runnerv2.RunnerService_ExecuteServer) error {
	ctx := srv.Context()
	logger := r.logger.With(zap.String("id", ulid.GenerateID()))

	logger.Info("running Execute in runnerService")

	// Get the initial request.
	req, err := srv.Recv()
	if err != nil {
		if errors.Is(err, io.EOF) {
			logger.Info("client closed the connection while getting initial request")
			return nil
		}
		logger.Info("failed to receive a request", zap.Error(err))
		return errors.WithStack(err)
	}
	logger.Info("received initial request", zap.Any("req", req))

	// Manage the session.
	session, existed, err := r.getOrCreateSessionFromRequest(req)
	if err != nil {
		return err
	}
	if !existed {
		r.sessions.Add(session)
	}

	if err := session.SetEnv(req.Config.Env...); err != nil {
		return err
	}

	// Load the project.
	// TODO(adamb): this should come from the runme.yaml in the future.
	proj, err := convertProtoProjectToProject(req.GetProject())
	if err != nil {
		return err
	}

	exec, err := newExecution(
		req.Config,
		proj,
		session,
		logger,
		req.StoreStdoutInEnv,
	)
	if err != nil {
		return err
	}

	// Start the command and send the initial response with PID.
	if err := exec.Cmd.Start(ctx); err != nil {
		return err
	}
	if err := srv.Send(&runnerv2.ExecuteResponse{
		Pid: &wrapperspb.UInt32Value{Value: uint32(exec.Cmd.Pid())},
	}); err != nil {
		return err
	}

	// From the initial request, only the config is used to create a new execution.
	// The rest of fields like InputData, Winsize, Stop are handled in this goroutine,
	// and then the goroutine continues to read the next requests.
	go func(initialReq *runnerv2.ExecuteRequest) {
		req := initialReq

		for {
			var err error

			if err := exec.SetWinsize(req.Winsize); err != nil {
				logger.Info("failed to set winsize; ignoring", zap.Error(err))
			}

			_, err = exec.Write(req.InputData)
			if err != nil {
				logger.Info("failed to write to stdin; ignoring", zap.Error(err))
			}

			if err := exec.Stop(req.Stop); err != nil {
				logger.Info("failed to stop program; ignoring", zap.Error(err))
			}

			req, err = srv.Recv()
			logger.Info("received request", zap.Any("req", req), zap.Error(err))
			switch {
			case err == nil:
				// continue
			case err == io.EOF:
				logger.Info("client closed its send direction; stopping the program")
				if err := exec.Cmd.Signal(os.Interrupt); err != nil {
					logger.Info("failed to stop the command with interrupt signal", zap.Error(err))
				}
				return
			case status.Convert(err).Code() == codes.Canceled || status.Convert(err).Code() == codes.DeadlineExceeded:
				if !exec.Cmd.Running() {
					logger.Info("stream canceled after the process finished; ignoring")
				} else {
					logger.Info("stream canceled while the process is still running; program will be stopped if non-background")
					if err := exec.Cmd.Signal(os.Kill); err != nil {
						logger.Info("failed to stop program with kill signal", zap.Error(err))
					}
				}
				return
			}
		}
	}(req)

	exitCode, waitErr := exec.Wait(ctx, srv)
	logger.Info("command finished", zap.Int("exitCode", exitCode), zap.Error(waitErr))

	var finalExitCode *wrapperspb.UInt32Value
	if exitCode > -1 {
		finalExitCode = wrapperspb.UInt32(uint32(exitCode))
	}

	if err := srv.Send(&runnerv2.ExecuteResponse{
		ExitCode: finalExitCode,
	}); err != nil {
		logger.Info("failed to send exit code", zap.Error(err))
	}

	return waitErr
}

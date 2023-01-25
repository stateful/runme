package runner

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/pkg/errors"
	runnerv1 "github.com/stateful/runme/internal/gen/proto/go/runme/runner/v1"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type runnerService struct {
	runnerv1.UnimplementedRunnerServiceServer

	mu       sync.RWMutex
	sessions []*session

	logger *zap.Logger
}

func NewRunnerService(logger *zap.Logger) runnerv1.RunnerServiceServer {
	return newRunnerService(logger)
}

func newRunnerService(logger *zap.Logger) *runnerService {
	return &runnerService{
		logger: logger,
	}
}

func toRunnerv1Session(sess *session) *runnerv1.Session {
	return &runnerv1.Session{
		Id:       sess.ID,
		Envs:     sess.Envs(),
		Metadata: sess.Metadata,
	}
}

func (r *runnerService) CreateSession(ctx context.Context, req *runnerv1.CreateSessionRequest) (*runnerv1.CreateSessionResponse, error) {
	r.logger.Info("running CreateSession in runnerService")

	r.mu.Lock()
	sess := newSession(req.Envs, r.logger)
	r.sessions = append(r.sessions, sess)
	r.mu.Unlock()

	return &runnerv1.CreateSessionResponse{
		Session: toRunnerv1Session(sess),
	}, nil
}

func (r *runnerService) GetSession(_ context.Context, req *runnerv1.GetSessionRequest) (*runnerv1.GetSessionResponse, error) {
	r.logger.Info("running GetSession in runnerService")

	r.mu.RLock()
	sess := r.findSession(req.Id)
	r.mu.RUnlock()

	if sess == nil {
		return nil, status.Error(codes.NotFound, "session not found")
	}

	return &runnerv1.GetSessionResponse{
		Session: toRunnerv1Session(sess),
	}, nil
}

func (r *runnerService) ListSessions(_ context.Context, req *runnerv1.ListSessionsRequest) (*runnerv1.ListSessionsResponse, error) {
	r.logger.Info("running ListSessions in runnerService")

	r.mu.RLock()
	sessions := make([]*runnerv1.Session, 0, len(r.sessions))
	for _, s := range r.sessions {
		sessions = append(sessions, toRunnerv1Session(s))
	}
	r.mu.RUnlock()

	return &runnerv1.ListSessionsResponse{Sessions: sessions}, nil
}

func (r *runnerService) DeleteSession(_ context.Context, req *runnerv1.DeleteSessionRequest) (*runnerv1.DeleteSessionResponse, error) {
	r.logger.Info("running DeleteSession in runnerService")

	deleted := false

	r.mu.Lock()
	for id, s := range r.sessions {
		if s.ID == req.Id {
			deleted = true
			if id == len(r.sessions)-1 {
				r.sessions = r.sessions[:id]
			} else {
				r.sessions = append(r.sessions[:id], r.sessions[id+1:]...)
			}
			break
		}
	}
	r.mu.Unlock()

	if !deleted {
		return nil, status.Error(codes.NotFound, "session not found")
	}
	return &runnerv1.DeleteSessionResponse{}, nil
}

func (r *runnerService) findSession(id string) *session {
	var sess *session
	for _, s := range r.sessions {
		if s.ID == id {
			sess = s
		}
	}
	return sess
}

func (r *runnerService) Execute(srv runnerv1.RunnerService_ExecuteServer) error {
	r.logger.Info("running Execute in runnerService")

	// Get the initial request.
	req, err := srv.Recv()
	if err != nil {
		r.logger.Info("failed to receive a request", zap.Error(err))
		return errors.WithStack(err)
	}

	cmd, err := newCommand(
		&commandConfig{
			ProgramName: req.ProgramName,
			Args:        req.Arguments,
			Directory:   req.Directory,
			Envs:        req.Envs,
			Tty:         req.Tty,
			IsShell:     true,
			Commands:    req.Commands,
			Script:      req.Script,
		},
		r.logger,
	)
	if err != nil {
		return err
	}

	go func() {
		for {
			req, err := srv.Recv()
			if err != nil {
				if err == io.EOF {
					r.logger.Info("client closed send direction", zap.Error(err))
				} else {
					r.logger.Info("failed to receive requests", zap.Error(err))
				}
				return
			}
			if len(req.InputData) == 0 {
				continue
			}
			_, err = cmd.Stdin.Write(req.InputData)
			if err != nil {
				r.logger.Info("failed to write to cmd.Stdin", zap.Error(err))
				return
			}
		}
	}()

	exitCode, err := executeCmd(
		srv.Context(),
		cmd,
		func(data output) error {
			return srv.Send(&runnerv1.ExecuteResponse{
				StdoutData: data.Stdout,
				StderrData: data.Stderr,
			})
		},
		time.Millisecond*250,
	)

	r.logger.Info("finished command execution", zap.Error(err), zap.Int("exitCode", exitCode))

	if exitCode > -1 {
		return srv.Send(&runnerv1.ExecuteResponse{
			ExitCode: wrapperspb.UInt32(uint32(exitCode)),
		})
	}

	return err
}

func executeCmd(
	ctx context.Context,
	cmd *command,
	processData func(output) error,
	processDataInterval time.Duration,
) (int, error) {
	if err := cmd.Start(ctx); err != nil {
		return -1, err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	g := new(errgroup.Group)
	outs := make(chan output)

	g.Go(func() error {
		err := readLoop(ctx, time.Second, cmd.Stdout, cmd.Stderr, outs)
		close(outs)
		if errors.Is(err, io.EOF) {
			err = nil
		}
		return err
	})

	g.Go(func() error {
		for data := range outs {
			if err := processData(data); err != nil {
				return err
			}
		}
		return nil
	})

	werr := cmd.Wait()
	if werr != nil {
		return exitCodeFromErr(werr), werr
	}
	cancel() // cancel ctx so that readLoop exits
	if err := g.Wait(); err != nil {
		return -1, err
	}
	return 0, nil
}

type output struct {
	Stdout []byte
	Stderr []byte
}

func (o output) Clone() (result output) {
	if len(o.Stdout) == 0 {
		result.Stdout = nil
	} else {
		result.Stdout = make([]byte, len(o.Stdout))
		copy(result.Stdout, o.Stdout)
	}
	if len(o.Stderr) == 0 {
		result.Stderr = nil
	} else {
		result.Stderr = make([]byte, len(o.Stderr))
		copy(result.Stderr, o.Stderr)
	}
	return
}

func readLoop(
	ctx context.Context,
	timeout time.Duration,
	stdout io.Reader,
	stderr io.Reader,
	results chan<- output,
) error {
	out1, err1 := make([]byte, 1024), make([]byte, 1024)
	out2, err2 := make([]byte, 1024), make([]byte, 1024)
	idx := 0

	read := func() error {
		outb, errb := out1, err1
		idx++
		if idx%2 == 0 {
			outb, errb = out2, err2
		}

		var result output

		n, err := stdout.Read(outb)
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		if n > 0 {
			result.Stdout = outb[:n]
		}

		n, err = stderr.Read(errb)
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		if n > 0 {
			result.Stderr = errb[:n]
		}

		if len(result.Stdout) > 0 || len(result.Stderr) > 0 {
			results <- result
		}

		return nil
	}

	for {
		select {
		case <-ctx.Done():
			if err := read(); err != nil {
				return err
			}
			return nil
		case <-time.After(timeout):
			if err := read(); err != nil {
				return err
			}
		}
	}
}

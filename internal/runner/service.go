package runner

import (
	"context"
	"io"
	"sync"

	"github.com/pkg/errors"
	runnerv1 "github.com/stateful/runme/internal/gen/proto/go/runme/runner/v1"
	"github.com/stateful/runme/internal/rbuffer"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const bufferSize = 10 * 1024

type runnerService struct {
	runnerv1.UnimplementedRunnerServiceServer

	mu       sync.RWMutex
	sessions []*Session

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

func toRunnerv1Session(sess *Session) *runnerv1.Session {
	return &runnerv1.Session{
		Id:       sess.ID,
		Envs:     sess.Envs(),
		Metadata: sess.Metadata,
	}
}

func (r *runnerService) CreateSession(ctx context.Context, req *runnerv1.CreateSessionRequest) (*runnerv1.CreateSessionResponse, error) {
	r.logger.Info("running CreateSession in runnerService")

	r.mu.Lock()
	sess := NewSession(req.Envs, r.logger)
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

func (r *runnerService) findSession(id string) *Session {
	var sess *Session
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
		if errors.Is(err, io.EOF) {
			r.logger.Info("client closed the connection while getting initial request")
			return nil
		}
		r.logger.Info("failed to receive a request", zap.Error(err))
		return errors.WithStack(err)
	}

	var sess *Session
	if req.SessionId != "" {
		sess = r.findSession(req.SessionId)
		if sess == nil {
			return errors.New("session not found")
		}
	} else {
		sess = NewSession(nil, r.logger)
	}

	if len(req.Envs) > 0 {
		sess.AddEnvs(req.Envs)
	}

	stdin, stdinWriter := io.Pipe()
	stdout := rbuffer.NewRingBuffer(bufferSize)
	stderr := rbuffer.NewRingBuffer(bufferSize)
	// Close buffers so that the readers will be notified about EOF.
	// It's ok to close the buffers multiple times.
	defer func() { _ = stdout.Close() }()
	defer func() { _ = stderr.Close() }()

	cmd, err := newCommand(
		&commandConfig{
			ProgramName: req.ProgramName,
			Args:        req.Arguments,
			Directory:   req.Directory,
			Session:     sess,
			Tty:         req.Tty,
			Stdin:       stdin,
			Stdout:      stdout,
			Stderr:      stderr,
			IsShell:     true,
			Commands:    req.Commands,
			Script:      req.Script,
			Logger:      r.logger,
		},
	)
	if err != nil {
		return err
	}

	if err := cmd.Start(srv.Context()); err != nil {
		return err
	}

	go func() {
		defer func() {
			_ = stdinWriter.Close()
		}()

		if len(req.InputData) > 0 {
			if _, err := stdinWriter.Write(req.InputData); err != nil {
				r.logger.Info("failed to write initial input to stdin", zap.Error(err))
				// TODO(adamb): we likely should communicate it to the client.
				// Then, the client could decide what to do.
				return
			}
		}

		for {
			req, err := srv.Recv()
			if err != nil {
				// If the error is io.EOF, then we assume that the client
				// closed the connection with an intention to stop execution.
				// If it is a different error, we try to stop the command.
				if err == io.EOF {
					r.logger.Info("client closed the connection")
				} else {
					r.logger.Info("failed to receive requests; stop program", zap.Error(err))
					if err := cmd.Stop(); err != nil {
						r.logger.Info("failed to stop program", zap.Error(err))
					}
				}
				return
			}

			// TODO(adamb): introduce a new field to forcefully stop the command.

			if len(req.InputData) != 0 {
				_, err = stdinWriter.Write(req.InputData)
				if err != nil {
					r.logger.Info("failed to write to stdin", zap.Error(err))
					// TODO(adamb): we likely should communicate it to the client.
					// Then, the client could decide what to do.
					return
				}
			}
		}
	}()

	g := new(errgroup.Group)
	datac := make(chan output)

	g.Go(func() error {
		for data := range datac {
			err := srv.Send(&runnerv1.ExecuteResponse{
				StdoutData: data.Stdout,
				StderrData: data.Stderr,
			})
			if err != nil {
				return err
			}
		}
		return nil
	})

	g.Go(func() error {
		err := readLoop(stdout, stderr, datac)
		close(datac)
		if errors.Is(err, io.EOF) {
			err = nil
		}
		return err
	})

	// Wait for the process to finish.
	werr := cmd.ProcessWait()
	exitCode := exitCodeFromErr(werr)
	if exitCode == -1 {
		r.logger.Info("process failed; continue with finalizer", zap.Error(err))
	}

	// Close the stdinWriter so that the loops in cmd will finish.
	// The problem occurs only with TTY.
	_ = stdinWriter.Close()

	if err := cmd.Finalize(); err != nil {
		r.logger.Info("command finalizer failed", zap.Error(err))
		if werr == nil {
			return err
		}
	}

	if exitCode == -1 {
		return werr
	}

	// Close buffers so that the readLoop() can exit.
	_ = stdout.Close()
	_ = stderr.Close()

	if err := g.Wait(); err != nil {
		r.logger.Info("failed to wait for goroutines to finish", zap.Error(err))
	}

	r.logger.Info("finished command execution", zap.Int("exitCode", exitCode))

	return srv.Send(&runnerv1.ExecuteResponse{
		ExitCode: wrapperspb.UInt32(uint32(exitCode)),
	})
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

// readLoop uses two sets of buffers in order to avoid allocating
// new memory over and over and putting more presure on GC.
// When the first set is read, it is sent to a channel called `results`.
// `results` should be an unbuffered channel. When a consumer consumes
// from the channel, the loop is unblocked and it moves on to read
// into the second set of buffers and blocks. During this time,
// the consumer has a chance to do something with the data stored
// in the first set of buffers.
func readLoop(
	stdout io.Reader,
	stderr io.Reader,
	results chan<- output,
) error {
	if cap(results) > 0 {
		panic("readLoop requires unbuffered channel")
	}

	read := func(reader io.Reader, fn func(p []byte) output) error {
		buf1, buf2 := make([]byte, bufferSize), make([]byte, bufferSize)
		idx := 0

		for {
			buf := buf1
			if idx = (idx + 1) % 2; idx == 0 {
				buf = buf2
			}
			n, err := reader.Read(buf)
			if err != nil {
				if errors.Is(err, io.EOF) {
					return nil
				}
				return errors.WithStack(err)
			} else if n > 0 {
				results <- fn(buf[:n])
			}
		}
	}

	g := new(errgroup.Group)

	g.Go(func() error {
		return read(stdout, func(p []byte) output {
			return output{Stdout: p}
		})
	})

	g.Go(func() error {
		return read(stderr, func(p []byte) output {
			return output{Stderr: p}
		})
	})

	return g.Wait()
}

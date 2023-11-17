package runner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/creack/pty"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"

	runnerv1 "github.com/stateful/runme/internal/gen/proto/go/runme/runner/v1"
	"github.com/stateful/runme/internal/project"
	"github.com/stateful/runme/internal/rbuffer"
	ulid "github.com/stateful/runme/internal/ulid"
)

const (
	MaxMsgSize = 4096 << 10 // 4 MiB

	// ringBufferSize limits the size of the ring buffers
	// that sit between a command and the handler.
	ringBufferSize = 8192 << 10 // 8 MiB

	// msgBufferSize limits the size of data chunks
	// sent by the handler to clients. It's smaller
	// intentionally as typically the messages are
	// small.
	// In the future, it might be worth to implement
	// variable-sized buffers.
	msgBufferSize = 32 << 10 // 32 KiB
)

type runnerService struct {
	runnerv1.UnimplementedRunnerServiceServer

	sessions *SessionList

	logger *zap.Logger
}

func NewRunnerService(logger *zap.Logger) (runnerv1.RunnerServiceServer, error) {
	return newRunnerService(logger)
}

func newRunnerService(logger *zap.Logger) (*runnerService, error) {
	sessions, err := NewSessionList()
	if err != nil {
		return nil, err
	}

	return &runnerService{
		logger:   logger,
		sessions: sessions,
	}, nil
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

	proj, err := ConvertRunnerProject(req.Project)
	if err != nil {
		return nil, err
	}

	envs := make([]string, len(req.Envs))
	copy(envs, req.Envs)

	if proj != nil {
		projEnvs, err := proj.LoadEnvs()
		if err != nil {
			return nil, err
		}

		envs = append(envs, projEnvs...)
	}

	sess, err := NewSession(envs, r.logger)
	if err != nil {
		return nil, err
	}

	r.sessions.AddSession(sess)

	return &runnerv1.CreateSessionResponse{
		Session: toRunnerv1Session(sess),
	}, nil
}

func (r *runnerService) GetSession(_ context.Context, req *runnerv1.GetSessionRequest) (*runnerv1.GetSessionResponse, error) {
	r.logger.Info("running GetSession in runnerService")

	sess, ok := r.sessions.GetSession(req.Id)

	if !ok {
		return nil, status.Error(codes.NotFound, "session not found")
	}

	return &runnerv1.GetSessionResponse{
		Session: toRunnerv1Session(sess),
	}, nil
}

func (r *runnerService) ListSessions(_ context.Context, req *runnerv1.ListSessionsRequest) (*runnerv1.ListSessionsResponse, error) {
	r.logger.Info("running ListSessions in runnerService")

	sessions, err := r.sessions.ListSessions()
	if err != nil {
		return nil, err
	}

	runnerSessions := make([]*runnerv1.Session, 0, len(sessions))
	for _, s := range sessions {
		runnerSessions = append(runnerSessions, toRunnerv1Session(s))
	}

	return &runnerv1.ListSessionsResponse{Sessions: runnerSessions}, nil
}

func (r *runnerService) DeleteSession(_ context.Context, req *runnerv1.DeleteSessionRequest) (*runnerv1.DeleteSessionResponse, error) {
	r.logger.Info("running DeleteSession in runnerService")

	deleted := r.sessions.DeleteSession(req.Id)

	if !deleted {
		return nil, status.Error(codes.NotFound, "session not found")
	}
	return &runnerv1.DeleteSessionResponse{}, nil
}

func (r *runnerService) findSession(id string) *Session {
	if sess, ok := r.sessions.GetSession(id); ok {
		return sess
	}

	return nil
}

func ConvertRunnerProject(runnerProj *runnerv1.Project) (*project.Project, error) {
	if runnerProj == nil {
		return nil, nil
	}

	opts := []project.ProjectOption{project.WithFindRepoUpward()}

	if len(runnerProj.EnvLoadOrder) > 0 {
		opts = append(opts, project.WithEnvFilesReadOrder(runnerProj.EnvLoadOrder))
	}

	proj, err := project.NewDirProject(runnerProj.Root, opts...)
	if err != nil {
		return nil, err
	}
	return proj, nil
}

func (r *runnerService) Execute(srv runnerv1.RunnerService_ExecuteServer) error {
	logger := r.logger.With(zap.String("_id", ulid.GenerateID()))

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

	logger.Debug("received initial request", zap.Any("req", req))

	createSession := func(envs []string) (*Session, error) {
		return NewSession(envs, r.logger)
	}

	var stdoutMem []byte
	storeStdout := req.StoreLastOutput

	var sess *Session
	switch req.SessionStrategy {
	case runnerv1.SessionStrategy_SESSION_STRATEGY_UNSPECIFIED:
		if req.SessionId != "" {
			sess = r.findSession(req.SessionId)
			if sess == nil {
				return errors.New("session not found")
			}
		} else {
			sess, err = createSession(nil)
			if err != nil {
				return err
			}
		}

		if len(req.Envs) > 0 {
			sess.AddEnvs(req.Envs)
		}
	case runnerv1.SessionStrategy_SESSION_STRATEGY_MOST_RECENT:
		sess, err = r.sessions.MostRecentOrCreate(func() (*Session, error) { return createSession(req.Envs) })
		if err != nil {
			return err
		}
	}

	stdin, stdinWriter := io.Pipe()
	stdout := rbuffer.NewRingBuffer(ringBufferSize)
	stderr := rbuffer.NewRingBuffer(ringBufferSize)
	// Close buffers so that the readers will be notified about EOF.
	// It's ok to close the buffers multiple times.
	defer func() { _ = stdout.Close() }()
	defer func() { _ = stderr.Close() }()

	cfg := &commandConfig{
		ProgramName:   req.ProgramName,
		Args:          req.Arguments,
		Directory:     req.Directory,
		Session:       sess,
		Tty:           req.Tty,
		Stdin:         stdin,
		Stdout:        stdout,
		Stderr:        stderr,
		Commands:      req.Commands,
		Script:        req.Script,
		Logger:        r.logger,
		Winsize:       runnerWinsizeToPty(req.Winsize),
		LanguageID:    req.LanguageId,
		FileExtension: req.FileExtension,
	}

	switch req.CommandMode {
	case runnerv1.CommandMode_COMMAND_MODE_UNSPECIFIED:
		cfg.CommandMode = CommandModeNone
	case runnerv1.CommandMode_COMMAND_MODE_INLINE_SHELL:
		cfg.CommandMode = CommandModeInlineShell
	case runnerv1.CommandMode_COMMAND_MODE_TEMP_FILE:
		cfg.CommandMode = CommandModeTempFile
	}

	if req.Project != nil {
		proj, err := ConvertRunnerProject(req.Project)
		if err != nil {
			return err
		}

		projEnvs, err := proj.LoadEnvs()
		if err != nil {
			return err
		}

		cfg.PreEnv = append(cfg.PreEnv, projEnvs...)
	}

	logger.Debug("command config", zap.Any("cfg", cfg))
	cmd, err := newCommand(cfg)
	if err != nil {
		var errInvalidLanguage ErrInvalidLanguage
		if errors.As(err, &errInvalidLanguage) {
			st := status.New(codes.InvalidArgument, "invalid LanguageId")
			v := &errdetails.BadRequest_FieldViolation{
				Field:       "LanguageId",
				Description: "unable to find program for language",
			}
			br := &errdetails.BadRequest{}
			br.FieldViolations = append(br.FieldViolations, v)
			st, err := st.WithDetails(br)
			if err != nil {
				return fmt.Errorf("unexpected error attaching metadata: %v", err)
			}

			return st.Err()
		}

		var errInvalidProgram ErrInvalidProgram
		if errors.As(err, &errInvalidProgram) {
			st := status.New(codes.InvalidArgument, "invalid ProgramName")
			v := &errdetails.BadRequest_FieldViolation{
				Field:       "ProgramName",
				Description: "unable to find program",
			}
			br := &errdetails.BadRequest{}
			br.FieldViolations = append(br.FieldViolations, v)
			st, err := st.WithDetails(br)
			if err != nil {
				return fmt.Errorf("unexpected error attaching metadata: %v", err)
			}

			return st.Err()
		}

		return err
	}

	cmdCtx := srv.Context()

	if req.Background {
		cmdCtx = context.Background()
	}

	if err := cmd.StartWithOpts(cmdCtx, &startOpts{}); err != nil {
		return err
	}

	if err := srv.Send(&runnerv1.ExecuteResponse{
		Pid: &runnerv1.ProcessPID{
			Pid: int64(cmd.cmd.Process.Pid),
		},
	}); err != nil {
		return err
	}

	// This goroutine will be closed when the handler exits or earlier.
	go func() {
		defer func() { _ = stdinWriter.Close() }()

		if len(req.InputData) > 0 {
			if _, err := stdinWriter.Write(req.InputData); err != nil {
				logger.Info("failed to write initial input to stdin", zap.Error(err))
				// TODO(adamb): we likely should communicate it to the client.
				// Then, the client could decide what to do.
				return
			}
		}

		// When TTY is false, it means that the command is run in non-interactive mode and
		// there will be no more input data.
		if !req.Tty {
			_ = stdinWriter.Close() // it's ok to close it multiple times
		}

		for {
			req, err := srv.Recv()
			if err == io.EOF {
				logger.Info("client closed the send direction; ignoring")
				return
			}
			if err != nil && status.Convert(err).Code() == codes.Canceled {
				if cmd.cmd.ProcessState != nil {
					logger.Info("stream canceled after the process finished; ignoring")
				} else {
					logger.Info("stream canceled while the process is still running; program will be stopped if non-background")
				}
				return
			}
			if err != nil {
				logger.Info("error while receiving a request; stopping the program", zap.Error(err))
				err := cmd.Kill()
				if err != nil {
					logger.Info("failed to stop program", zap.Error(err))
				}
				return
			}

			if req.Stop != runnerv1.ExecuteStop_EXECUTE_STOP_UNSPECIFIED {
				logger.Info("requested the program to stop")

				var err error

				switch req.Stop {
				case runnerv1.ExecuteStop_EXECUTE_STOP_INTERRUPT:
					err = cmd.StopWithSignal(os.Interrupt)
				case runnerv1.ExecuteStop_EXECUTE_STOP_KILL:
					err = cmd.Kill()
				}

				if err != nil {
					logger.Info("failed to stop program on request", zap.Error(err), zap.Any("signal", req.Stop))
				}

				return
			}

			if len(req.InputData) != 0 {
				logger.Debug("received input data", zap.Int("len", len(req.InputData)))
				_, err = stdinWriter.Write(req.InputData)
				if err != nil {
					logger.Info("failed to write to stdin", zap.Error(err))
					// TODO(adamb): we likely should communicate it to the client.
					// Then, the client could decide what to do.
					return
				}
			}

			// only update winsize when field is explicitly set
			if req.ProtoReflect().Has(
				req.ProtoReflect().Descriptor().Fields().ByName("winsize"),
			) {
				cmd.setWinsize(runnerWinsizeToPty(req.Winsize))
			}
		}
	}()

	g := new(errgroup.Group)
	datac := make(chan output)

	g.Go(func() error {
		err := readLoop(stdout, stderr, datac)
		close(datac)
		if errors.Is(err, io.EOF) {
			err = nil
		}
		return err
	})

	g.Go(func() error {
		for data := range datac {
			logger.Debug("sending data", zap.Int("lenStdout", len(data.Stdout)), zap.Int("lenStderr", len(data.Stderr)))
			err := srv.Send(&runnerv1.ExecuteResponse{
				StdoutData: data.Stdout,
				StderrData: data.Stderr,
			})
			if err != nil {
				return err
			}

			if storeStdout && len(stdoutMem) < maxEnvSize {
				// sanitize for environment variable
				sanitized := bytes.ReplaceAll(data.Stdout, []byte{'\000'}, []byte{})
				stdoutMem = append(stdoutMem, sanitized...)
			}
		}
		return nil
	})

	// Wait for the process to finish.
	werr := cmd.ProcessWait()
	exitCode := exitCodeFromErr(werr)

	logger.Info("command finished", zap.Int("exitCode", exitCode))

	// Close the stdinWriter so that the loops in the `cmd` will finish.
	// The problem occurs only with TTY.
	_ = stdinWriter.Close()

	if err := cmd.Finalize(); err != nil {
		logger.Info("command finalizer failed", zap.Error(err))
		if werr == nil {
			return err
		}
	}

	logger.Info("command was finalized successfully")

	// Close buffers so that the readLoop() can exit.
	_ = stdout.Close()
	_ = stderr.Close()

	werr = g.Wait()
	if werr != nil {
		logger.Info("failed to wait for goroutines to finish", zap.Error(err))
	}

	if storeStdout {
		_, err := sess.envStore.Set("__", string(stdoutMem))
		if err != nil {
			logger.Sugar().Errorf("%v", err)
		}
	}

	var finalExitCode *wrapperspb.UInt32Value
	if exitCode > -1 {
		finalExitCode = wrapperspb.UInt32(uint32(exitCode))
		logger.Info("sending the final response with exit code", zap.Int("exitCode", int(finalExitCode.GetValue())))
	} else {
		logger.Info("sending the final response without exit code since its unknown", zap.Int("exitCode", exitCode))
	}

	if err := srv.Send(&runnerv1.ExecuteResponse{
		ExitCode: finalExitCode,
	}); err != nil {
		logger.Info("failed to send exit code", zap.Error(err))
		if werr == nil {
			werr = err
		}
	}

	return werr
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
		for {
			buf := make([]byte, msgBufferSize)
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

func runnerWinsizeToPty(winsize *runnerv1.Winsize) *pty.Winsize {
	if winsize == nil {
		// sane default
		return &pty.Winsize{Cols: 80}
	}

	return &pty.Winsize{
		Rows: uint16(winsize.Rows),
		Cols: uint16(winsize.Cols),
		X:    uint16(winsize.X),
		Y:    uint16(winsize.Y),
	}
}

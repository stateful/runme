package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/creack/pty"
	"github.com/gabriel-vasile/mimetype"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"

	commandpkg "github.com/stateful/runme/v3/internal/command"
	"github.com/stateful/runme/v3/internal/owl"
	"github.com/stateful/runme/v3/internal/rbuffer"
	"github.com/stateful/runme/v3/internal/ulid"
	runnerv1 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/runner/v1"
	"github.com/stateful/runme/v3/pkg/project"
)

const (
	// ringBufferSize limits the size of the ring buffers
	// that sit between a command and the handler.
	ringBufferSize = 16 * 1000 * 1000 // 16 MB

	// msgBufferSize limits the size of data chunks
	// sent by the handler to clients. It's smaller
	// intentionally as typically the messages are
	// small.
	// In the future, it might be worth to implement
	// variable-sized buffers.
	msgBufferSize = 32 * 1000 // 32 KB
)

// Only allow uppercase letters, digits and underscores, min three chars
var OpininatedEnvVarNamingRegexp = regexp.MustCompile(`^[A-Z_][A-Z0-9_]{1}[A-Z0-9_]*[A-Z][A-Z0-9_]*$`)

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

func toRunnerv1Session(sess *Session) (*runnerv1.Session, error) {
	env, err := sess.Envs()
	if err != nil {
		return nil, err
	}
	return &runnerv1.Session{
		Id:       sess.ID,
		Envs:     env,
		Metadata: sess.Metadata,
	}, nil
}

func (r *runnerService) CreateSession(ctx context.Context, req *runnerv1.CreateSessionRequest) (*runnerv1.CreateSessionResponse, error) {
	r.logger.Info("running CreateSession in runnerService")

	proj, err := ConvertRunnerProject(req.Project)
	if err != nil {
		return nil, err
	}

	envs := make([]string, len(req.Envs))
	copy(envs, req.Envs)

	owlStore := req.EnvStoreType == runnerv1.SessionEnvStoreType_SESSION_ENV_STORE_TYPE_OWL

	// todo(sebastian): perhaps we should move loading logic into session, like for owl store
	if proj != nil && !owlStore {
		projEnvs, err := proj.LoadEnv()
		if err != nil {
			return nil, err
		}

		envs = append(envs, projEnvs...)
	}

	sess, err := NewSessionWithStore(envs, proj, owlStore, r.logger)
	if err != nil {
		return nil, err
	}

	r.sessions.AddSession(sess)

	r.logger.Debug("created session", zap.String("id", sess.ID))

	runnerSess, err := toRunnerv1Session(sess)
	if err != nil {
		return nil, err
	}
	return &runnerv1.CreateSessionResponse{
		Session: runnerSess,
	}, nil
}

func (r *runnerService) GetSession(_ context.Context, req *runnerv1.GetSessionRequest) (*runnerv1.GetSessionResponse, error) {
	r.logger.Info("running GetSession in runnerService")

	sess, ok := r.sessions.GetSession(req.Id)

	if !ok {
		return nil, status.Error(codes.NotFound, "session not found")
	}

	runnerSess, err := toRunnerv1Session(sess)
	if err != nil {
		return nil, err
	}
	return &runnerv1.GetSessionResponse{
		Session: runnerSess,
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
		runnerSess, err := toRunnerv1Session(s)
		if err != nil {
			return nil, err
		}
		runnerSessions = append(runnerSessions, runnerSess)
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

	// todo(sebastian): this is not right for IDE projects - does it matter for others?
	// opts := []project.ProjectOption{project.WithFindRepoUpward()}
	opts := []project.ProjectOption{}

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
	_id := ulid.GenerateID()
	logger := r.logger.With(zap.String("_id", _id))

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

	execInfo := &commandpkg.ExecutionInfo{
		RunID:     _id,
		KnownName: req.GetKnownName(),
		KnownID:   req.GetKnownId(),
	}
	ctx := commandpkg.ContextWithExecutionInfo(srv.Context(), execInfo)

	if req.KnownId != "" {
		logger = logger.With(zap.String("knownID", req.KnownId))
	}
	if req.KnownName != "" {
		logger = logger.With(zap.String("knownName", req.KnownName))
	}
	logger.Debug("received initial request", zap.Any("req", zapProto(req, logger)))

	createSession := func(envs []string) (*Session, error) {
		// todo(sebastian): owl store?
		return NewSession(envs, nil, r.logger)
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
			err := sess.AddEnvs(ctx, req.Envs)
			if err != nil {
				return err
			}
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
	case runnerv1.CommandMode_COMMAND_MODE_TERMINAL:
		return status.Error(codes.Unimplemented, "terminal mode is not supported")
	}

	logger.Debug("command config", zap.Any("cfg", cfg))
	cmd, err := newCommand(ctx, cfg)
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

	cmdCtx := ctx

	if req.Background {
		cmdCtx = commandpkg.ContextWithExecutionInfo(context.Background(), execInfo)
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
				if cmd.ProcessFinished() {
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
		firstStdoutSent := false

		for data := range datac {
			logger.Debug("sending data", zap.Int("lenStdout", len(data.Stdout)), zap.Int("lenStderr", len(data.Stderr)))

			resp := &runnerv1.ExecuteResponse{
				StdoutData: data.Stdout,
				StderrData: data.Stderr,
			}

			if !firstStdoutSent && len(data.Stdout) > 0 {
				if detected := mimetype.Detect(data.Stdout); detected != nil {
					resp.MimeType = detected.String()
				}
			}

			err := srv.Send(resp)
			if err != nil {
				return err
			}

			if len(resp.StdoutData) > 0 {
				firstStdoutSent = true
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
		err := sess.SetEnv(ctx, commandpkg.StoreStdoutEnvName, string(stdoutMem))
		if err != nil {
			logger.Sugar().Errorf("%v", err)
		}

		knownName := req.GetKnownName()
		if knownName != "" && runnerConformsOpinionatedEnvVarNaming(knownName) {
			err = sess.SetEnv(ctx, knownName, string(stdoutMem))
			if err != nil {
				logger.Warn("failed to set env", zap.Error(err))
			}
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

// zapProto is a helper function to be able to log protos as JSON objects.
// We want protos to be logged using the proto json format so we can deserialize them from the logs.
// If you just log a proto with zap it will use the json serialization of the GoLang struct which will not match
// the proto json format. So we serialize the request to JSON and then deserialize it to a map so we can log it as a
// JSON object. A more efficient solution would be to use https://github.com/kazegusuri/go-proto-zap-marshaler
// to generate a custom zapcore.ObjectMarshaler implementation for each proto message.
func zapProto(pb proto.Message, logger *zap.Logger) map[string]interface{} {
	reqObj := map[string]interface{}{}
	reqJSON, err := protojson.Marshal(pb)
	if err != nil {
		logger.Error("failed to marshal request", zap.Error(err))
		reqObj["error"] = err.Error()
		return reqObj
	}

	if err := json.Unmarshal(reqJSON, &reqObj); err != nil {
		logger.Error("failed to unmarshal request", zap.Error(err))
		reqObj["error"] = err.Error()
	}

	return reqObj
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
// new memory over and over and putting more pressure on GC.
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
		return &pty.Winsize{Rows: 24, Cols: 80}
	}

	return &pty.Winsize{
		Rows: uint16(winsize.Rows),
		Cols: uint16(winsize.Cols),
		X:    uint16(winsize.X),
		Y:    uint16(winsize.Y),
	}
}

func runnerConformsOpinionatedEnvVarNaming(knownName string) bool {
	return OpininatedEnvVarNamingRegexp.MatchString(knownName)
}

func (r *runnerService) ResolveProgram(ctx context.Context, req *runnerv1.ResolveProgramRequest) (*runnerv1.ResolveProgramResponse, error) {
	r.logger.Info("running ResolveProgram in runnerService")

	// todo(sebastian): reenable once extension includes it in request
	// if req.GetLanguageId() == "" {
	// 	return nil, status.Error(codes.InvalidArgument, "language id is required")
	// }

	resolver, err := r.getProgramResolverFromReq(req)
	if err != nil {
		return nil, err
	}

	var modifiedScriptBuf bytes.Buffer

	script := req.GetScript()
	if commands := req.GetCommands(); script == "" && len(commands.Lines) > 0 {
		script = strings.Join(commands.Lines, "\n")
	}

	if script == "" {
		return nil, status.Error(codes.InvalidArgument, "either script or commands must be provided")
	}

	// todo(sebastian): figure out how to return commands
	response := &runnerv1.ResolveProgramResponse{
		Script: script,
	}

	// return early if it's not a shell language
	if !IsShellLanguage(req.GetLanguageId()) {
		return response, nil
	}

	result, err := resolver.Resolve(strings.NewReader(script), &modifiedScriptBuf)
	if err != nil {
		return nil, err
	}
	response.Script = modifiedScriptBuf.String()

	for _, item := range result.Variables {
		ritem := &runnerv1.ResolveProgramResponse_VarResult{
			Name:          item.Name,
			OriginalValue: item.OriginalValue,
			ResolvedValue: item.Value,
		}

		switch item.Status {
		case commandpkg.ProgramResolverStatusResolved:
			ritem.Status = runnerv1.ResolveProgramResponse_STATUS_RESOLVED
		case commandpkg.ProgramResolverStatusUnresolvedWithMessage:
			ritem.Status = runnerv1.ResolveProgramResponse_STATUS_UNRESOLVED_WITH_MESSAGE
		case commandpkg.ProgramResolverStatusUnresolvedWithPlaceholder:
			ritem.Status = runnerv1.ResolveProgramResponse_STATUS_UNRESOLVED_WITH_PLACEHOLDER
		case commandpkg.ProgramResolverStatusUnresolvedWithSecret:
			ritem.Status = runnerv1.ResolveProgramResponse_STATUS_UNRESOLVED_WITH_SECRET
		default:
			ritem.Status = runnerv1.ResolveProgramResponse_STATUS_UNSPECIFIED
		}

		response.Vars = append(response.Vars, ritem)
	}

	return response, nil
}

type requestWithSession interface {
	GetSessionId() string
	GetSessionStrategy() runnerv1.SessionStrategy
}

func (r *runnerService) getProgramResolverFromReq(req *runnerv1.ResolveProgramRequest) (*commandpkg.ProgramResolver, error) {
	// Add explicitly passed env as a source.
	sources := []commandpkg.ProgramResolverSource{
		commandpkg.ProgramResolverSourceFunc(req.Env),
	}

	// Add project env as a source.
	proj, err := ConvertRunnerProject(req.Project)
	if err != nil {
		return nil, err
	}
	if proj != nil {
		projEnvs, err := proj.LoadEnv()
		if err != nil {
			r.logger.Info("failed to load envs for project", zap.Error(err))
		} else {
			sources = append(sources, commandpkg.ProgramResolverSourceFunc(projEnvs))
		}
	}

	// Add session env as a source and pass info about sensitive env vars.
	sensitiveEnvKeys := []string{}
	session, found := r.getSessionFromRequest(req)
	if found {
		env, err := session.Envs()
		if err != nil {
			return nil, err
		}
		sources = append(sources, commandpkg.ProgramResolverSourceFunc(env))

		sensitiveEnvKeys, err = session.SensitiveEnvKeys()
		if err != nil {
			return nil, err
		}
	}

	mode := commandpkg.ProgramResolverModeAuto

	switch req.GetMode() {
	case runnerv1.ResolveProgramRequest_MODE_PROMPT_ALL:
		mode = commandpkg.ProgramResolverModePromptAll
	case runnerv1.ResolveProgramRequest_MODE_SKIP_ALL:
		mode = commandpkg.ProgramResolverModeSkipAll
	}

	return commandpkg.NewProgramResolver(mode, sensitiveEnvKeys, sources...), err
}

func (r *runnerService) getSessionFromRequest(req requestWithSession) (*Session, bool) {
	switch req.GetSessionStrategy() {
	case runnerv1.SessionStrategy_SESSION_STRATEGY_MOST_RECENT:
		return r.sessions.MostRecent()
	default:
		if sessID := req.GetSessionId(); sessID != "" {
			sess := r.findSession(sessID)
			return sess, sess != nil
		}
		return nil, false
	}
}

func (r *runnerService) MonitorEnvStore(req *runnerv1.MonitorEnvStoreRequest, srv runnerv1.RunnerService_MonitorEnvStoreServer) error {
	if req.Session == nil {
		return status.Error(codes.InvalidArgument, "session is required")
	}

	sess, ok := r.sessions.GetSession(req.Session.Id)
	if !ok {
		return status.Error(codes.NotFound, "session not found")
	}

	ctx, cancel := context.WithCancel(srv.Context())
	snapshotc := make(chan owl.SetVarItems)
	errc := make(chan error, 1)
	defer close(errc)
	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for snapshot := range snapshotc {
			msg := &runnerv1.MonitorEnvStoreResponse{
				Type: runnerv1.MonitorEnvStoreType_MONITOR_ENV_STORE_TYPE_SNAPSHOT,
			}

			if err := convertToMonitorEnvStoreResponse(msg, snapshot); err != nil {
				errc <- err
				goto errhandler
			}

			if err := srv.Send(msg); err != nil {
				errc <- err
				goto errhandler
			}

			continue

		errhandler:
			cancel()
			// subscribers should be notified that they should exit early
			// via cancel(). snapshotc will be closed, but it should be drained too
			// in order to clean up any in-flight results.
			// In theory, this is not necessary provided that all sends to snapshotc
			// are wrapped in selects which observe ctx.Done().
			//revive:disable:empty-block
			for range snapshotc {
			}
			//revive:enable:empty-block
		}

		errc <- nil
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := sess.Subscribe(ctx, snapshotc); err != nil {
			errc <- err
		}
	}()

	wg.Wait()

	return <-errc
}

func convertToMonitorEnvStoreResponse(msg *runnerv1.MonitorEnvStoreResponse, snapshot owl.SetVarItems) error {
	envsSnapshot := make([]*runnerv1.MonitorEnvStoreResponseSnapshot_SnapshotEnv, 0, len(snapshot))

	for _, item := range snapshot {
		status := runnerv1.MonitorEnvStoreResponseSnapshot_STATUS_UNSPECIFIED
		// todo(sebastian): once more final use enums in SetVarResult
		switch item.Value.Status {
		case "HIDDEN":
			status = runnerv1.MonitorEnvStoreResponseSnapshot_STATUS_HIDDEN
		case "MASKED":
			status = runnerv1.MonitorEnvStoreResponseSnapshot_STATUS_MASKED
		case "LITERAL":
			status = runnerv1.MonitorEnvStoreResponseSnapshot_STATUS_LITERAL
		default:
			// noop
		}
		es := &runnerv1.MonitorEnvStoreResponseSnapshot_SnapshotEnv{
			Name:          item.Var.Key,
			Spec:          item.Spec.Name,
			IsRequired:    item.Spec.Required,
			Origin:        item.Var.Origin,
			OriginalValue: item.Value.Original,
			ResolvedValue: item.Value.Resolved,
			Status:        status,
			CreateTime:    item.Var.Created.Format(time.RFC3339),
			UpdateTime:    item.Var.Updated.Format(time.RFC3339),
			Errors:        []*runnerv1.MonitorEnvStoreResponseSnapshot_Error{},
		}
		for _, verr := range item.Errors {
			if verr.Code < 0 {
				return fmt.Errorf("negative error code: %d", verr.Code)
			}
			es.Errors = append(es.Errors, &runnerv1.MonitorEnvStoreResponseSnapshot_Error{
				Code:    uint32(verr.Code),
				Message: verr.Message,
			})
		}
		envsSnapshot = append(envsSnapshot, es)
	}

	msg.Data = &runnerv1.MonitorEnvStoreResponse_Snapshot{
		Snapshot: &runnerv1.MonitorEnvStoreResponseSnapshot{
			Envs: envsSnapshot,
		},
	}

	return nil
}

package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	"go.uber.org/zap"

	"github.com/stateful/runme/v3/internal/ansi"
	"github.com/stateful/runme/v3/internal/lru"
	"github.com/stateful/runme/v3/internal/owl"
	rcontext "github.com/stateful/runme/v3/internal/runner/context"
	"github.com/stateful/runme/v3/internal/ulid"
	"github.com/stateful/runme/v3/pkg/project"
)

var owlStoreDefault = false

type envStorer interface {
	getEnv(string) (string, error) // Get
	envs() ([]string, error)       // Items
	addEnvs(context context.Context, envs []string) error
	updateStore(context context.Context, envs []string, newOrUpdated []string, deleted []string) error
	setEnv(context context.Context, k string, v string) error // Set
	sensitiveEnvKeys() ([]string, error)
	subscribe(ctx context.Context, snapshotc chan<- owl.SetVarItems) error
	complete()
}

// Session is an abstract entity separate from
// an execution. Currently, its main role is to
// keep track of environment variables.
type Session struct {
	ID        string
	Metadata  map[string]string
	envStorer envStorer

	logger *zap.Logger
}

func NewSession(envs []string, proj *project.Project, logger *zap.Logger) (*Session, error) {
	return NewSessionWithStore(envs, proj, owlStoreDefault, logger)
}

func NewSessionWithStore(envs []string, proj *project.Project, owlStore bool, logger *zap.Logger) (*Session, error) {
	sessionEnvs := []string(envs)

	var storer envStorer
	if owlStore && proj != nil {
		logger.Info("using owl store")
		var err error
		storer, err = newOwlStorer(sessionEnvs, proj, logger)
		if err != nil {
			return nil, err
		}
	} else {
		if proj == nil {
			logger.Debug("owl store requires project in session")
		}
		logger.Debug("using simple env store")
		storer = newRunnerStorer(sessionEnvs...)
	}

	s := &Session{
		ID:        ulid.GenerateID(),
		envStorer: storer,

		logger: logger,
	}
	return s, nil
}

func (s *Session) Identifer() string {
	return s.ID
}

func (s *Session) UpdateStore(context context.Context, envs []string, newOrUpdated []string, deleted []string) error {
	return s.envStorer.updateStore(context, envs, newOrUpdated, deleted)
}

func (s *Session) AddEnvs(context context.Context, envs []string) error {
	return s.envStorer.addEnvs(context, envs)
}

func (s *Session) SensitiveEnvKeys() ([]string, error) {
	return s.envStorer.sensitiveEnvKeys()
}

func (s *Session) SetEnv(context context.Context, k string, v string) error {
	return s.envStorer.setEnv(context, k, v)
}

func (s *Session) Envs() ([]string, error) {
	vals, err := s.envStorer.envs()
	if err != nil {
		return nil, err
	}
	return vals, nil
}

func (s *Session) Subscribe(ctx context.Context, snapshotc chan<- owl.SetVarItems) error {
	return s.envStorer.subscribe(ctx, snapshotc)
}

func (s *Session) Complete() {
	s.envStorer.complete()
}

func (s *Session) LoadDirEnv(ctx context.Context, proj *project.Project, directory string) (string, error) {
	if s == nil {
		return "", fmt.Errorf("session is nil")
	}

	if proj == nil || !proj.EnvDirEnvEnabled() {
		return "", nil
	}

	preEnv, err := proj.LoadEnv()
	if err != nil {
		return "", err
	}

	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	cfg := &ExecutableConfig{
		Dir:     directory,
		Name:    "LoadDirEnv",
		PreEnv:  preEnv,
		Session: s,
		Stderr:  stderr,
		Stdout:  stdout,
		Tty:     false,
	}

	const sourceDirEnv = "which direnv && eval $(direnv export $SHELL)"
	exec := &Shell{
		ExecutableConfig: cfg,
		Cmds:             []string{sourceDirEnv},
	}

	const dirEnvRc = ".envrc"
	rctx := rcontext.WithExecutionInfo(ctx, &rcontext.ExecutionInfo{
		RunID:       ulid.GenerateID(),
		ExecContext: dirEnvRc,
	})

	if err = exec.Run(rctx); err != nil {
		// skip errors caused by clients creating a new session on delete running on shutdown
		if errors.Is(err, context.Canceled) {
			return err.Error(), nil
		}

		// this means direnv isn't installed == not an error
		if exec.ExitCode() > 0 && bytes.Contains(stdout.Bytes(), []byte("not found")) {
			return "direnv not found", nil
		}

		return "", err
	}

	msg := ""
	if stderr.Len() > 0 {
		msg = string(bytes.Trim(ansi.Strip(stderr.Bytes()), "\r\n"))
	}

	return msg, nil
}

type runnerEnvStorer struct {
	// logger   *zap.Logger
	envStore *envStore
}

func newRunnerStorer(sessionEnvs ...string) *runnerEnvStorer {
	return &runnerEnvStorer{
		envStore: newEnvStore(sessionEnvs...),
	}
}

func (es *runnerEnvStorer) subscribe(_ context.Context, snapshotc chan<- owl.SetVarItems) error {
	defer close(snapshotc)
	return fmt.Errorf("not available for runner env store")
}

func (es *runnerEnvStorer) complete() {
	// noop
}

func (es *runnerEnvStorer) addEnvs(_ context.Context, envs []string) error {
	es.envStore.Add(envs...)
	return nil
}

func (es *runnerEnvStorer) sensitiveEnvKeys() ([]string, error) {
	// noop, not supported
	return []string{}, nil
}

func (es *runnerEnvStorer) getEnv(name string) (string, error) {
	return es.envStore.Get(name), nil
}

func (es *runnerEnvStorer) envs() ([]string, error) {
	envs, err := es.envStore.Values()
	if err != nil {
		return nil, err
	}
	return envs, nil
}

func (es *runnerEnvStorer) setEnv(_ context.Context, k string, v string) error {
	_, err := es.envStore.Set(k, v)
	return err
}

func (es *runnerEnvStorer) updateStore(_ context.Context, envs []string, newOrUpdated []string, deleted []string) error {
	es.envStore = newEnvStore(envs...).Add(newOrUpdated...).Delete(deleted...)
	return nil
}

type owlEnvStorerSubscriber chan<- owl.SetVarItems

type owlEnvStorer struct {
	logger   *zap.Logger
	owlStore *owl.Store

	mu          sync.RWMutex
	subscribers []owlEnvStorerSubscriber
}

func newOwlStorer(envs []string, proj *project.Project, logger *zap.Logger) (*owlEnvStorer, error) {
	// todo(sebastian): technically system should be session
	opts := []owl.StoreOption{
		owl.WithLogger(logger),
		owl.WithEnvs("[system]", envs...),
	}

	envSpecFiles := []string{}
	// envFilesOrder := []string{}
	if proj != nil {
		// todo(sebastian): specs loading should be independent of project
		envSpecFiles = []string{".env.sample", ".env.example", ".env.spec"}
	}

	for _, specFile := range envSpecFiles {
		raw, _ := proj.LoadRawFile(specFile)
		if raw == nil {
			continue
		}
		opt := owl.WithEnvFile(specFile, raw)
		if slices.Contains(envSpecFiles, specFile) {
			opt = owl.WithSpecFile(specFile, raw)
		}
		opts = append(opts, opt)
	}

	envWithSource, err := proj.LoadEnvWithSource()
	if err != nil {
		return nil, err
	}

	for envSource, envMap := range envWithSource {
		envs := []string{}
		for k, v := range envMap {
			env := fmt.Sprintf("%s=%s", k, v)
			envs = append(envs, env)
		}
		opts = append(opts, owl.WithEnvs(envSource, envs...))
	}

	owlYAML, err := proj.LoadRawFile(".runme/owl.yaml")
	if err != nil {
		return nil, err
	} else if owlYAML != nil {
		opts = append([]owl.StoreOption{
			owl.WithSpecDefsCRD(owlYAML),
			owl.WithResolutionCRD(owlYAML),
		}, opts...)
	}

	if owlYAML != nil {
		resolverOwlStore, err := owl.NewStore(opts...)
		if err != nil {
			return nil, err
		}

		logger.Debug("Resolving env external to the graph")
		if snapshot, err := resolverOwlStore.InsecureResolve(); err == nil {
			resolved := []string{}
			for _, item := range snapshot {
				if item.Value.Status != "LITERAL" {
					continue
				}
				resolved = append(resolved, fmt.Sprintf("%s=%s", item.Var.Key, item.Value.Resolved))
			}
			opts = append(opts, owl.WithEnvs("[gcp:secrets]", resolved...))
		} else {
			logger.Error("failed to resolve owl store", zap.Error(err))
		}
	}

	owlStore, err := owl.NewStore(opts...)
	if err != nil {
		return nil, err
	}

	return &owlEnvStorer{
		logger:   logger,
		owlStore: owlStore,
	}, nil
}

func (es *owlEnvStorer) subscribe(context context.Context, snapshotc chan<- owl.SetVarItems) error {
	defer es.mu.Unlock()
	es.mu.Lock()
	es.logger.Debug("subscribed to owl store")

	es.subscribers = append(es.subscribers, snapshotc)

	go func() {
		<-context.Done()
		err := es.unsubscribe(snapshotc)
		if err != nil {
			es.logger.Error("unsubscribe from owl store failed", zap.Error(err))
		}
	}()

	// avoid deadlock
	go func() {
		es.notifySubscribers()
	}()

	return nil
}

func (es *owlEnvStorer) complete() {
	defer es.mu.Unlock()
	es.mu.Lock()

	for _, sub := range es.subscribers {
		err := es.unsubscribeUnsafe(sub)
		if err != nil {
			es.logger.Error("unsubscribe from owl store failed", zap.Error(err))
		}
	}
}

func (es *owlEnvStorer) unsubscribe(snapshotc chan<- owl.SetVarItems) error {
	defer es.mu.Unlock()
	es.mu.Lock()

	return es.unsubscribeUnsafe(snapshotc)
}

func (es *owlEnvStorer) unsubscribeUnsafe(snapshotc chan<- owl.SetVarItems) error {
	es.logger.Debug("unsubscribed from owl store")

	for i, sub := range es.subscribers {
		if sub == snapshotc {
			es.subscribers = append(es.subscribers[:i], es.subscribers[i+1:]...)
			close(sub)
			return nil
		}
	}

	return fmt.Errorf("unknown subscriber")
}

func (es *owlEnvStorer) notifySubscribers() {
	defer es.mu.RUnlock()
	es.mu.RLock()

	snapshot, err := es.owlStore.Snapshot()
	if err != nil {
		es.logger.Error("failed to get snapshot", zap.Error(err))
		return
	}

	for _, sub := range es.subscribers {
		sub <- snapshot
	}
}

func (es *owlEnvStorer) updateStore(context context.Context, envs []string, newOrUpdated []string, deleted []string) error {
	if err := es.owlStore.Update(context, newOrUpdated, deleted); err != nil {
		return err
	}
	es.notifySubscribers()
	return nil
}

func (es *owlEnvStorer) addEnvs(context context.Context, envs []string) error {
	if err := es.owlStore.Update(context, envs, nil); err != nil {
		return err
	}
	es.notifySubscribers()
	return nil
}

func (es *owlEnvStorer) getEnv(name string) (string, error) {
	v, _, err := es.owlStore.InsecureGet(name)
	return v, err
}

func (es *owlEnvStorer) sensitiveEnvKeys() ([]string, error) {
	vals, err := es.owlStore.SensitiveKeys()
	if err != nil {
		return nil, err
	}
	return vals, nil
}

func (es *owlEnvStorer) setEnv(context context.Context, k string, v string) error {
	// todo(sebastian): add checking env length inside Update
	err := es.owlStore.Update(context, []string{fmt.Sprintf("%s=%s", k, v)}, nil)
	if err != nil {
		return err
	}
	es.notifySubscribers()
	return err
}

func (es *owlEnvStorer) envs() ([]string, error) {
	vals, err := es.owlStore.InsecureValues()
	if err != nil {
		return nil, err
	}
	return vals, nil
}

// SessionListCapacity is a maximum number of entries
// stored in a single SessionList.
const SessionListCapacity = 1024

func NewSessionList() *lru.Cache[*Session] {
	return lru.NewCache[*Session](SessionListCapacity)
}

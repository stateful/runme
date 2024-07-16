package runner

import (
	"context"
	"fmt"
	"sync"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/stateful/runme/v3/internal/owl"
	"github.com/stateful/runme/v3/internal/ulid"
	"github.com/stateful/runme/v3/pkg/project"
	"go.uber.org/zap"
)

var owlStoreDefault = false

type envStorer interface {
	getEnv(string) (string, error)
	envs() ([]string, error)
	sensitiveEnvKeys() ([]string, error)
	addEnvs(context context.Context, envs []string) error
	updateStore(context context.Context, envs []string, newOrUpdated []string, deleted []string) error
	setEnv(context context.Context, k string, v string) error
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
	if !owlStore {
		logger.Debug("using simple env store")
		storer = newRunnerStorer(sessionEnvs...)
	} else {
		logger.Info("using owl store")
		var err error
		storer, err = newOwlStorer(sessionEnvs, proj, logger)
		if err != nil {
			return nil, err
		}
	}

	s := &Session{
		ID:        ulid.GenerateID(),
		envStorer: storer,

		logger: logger,
	}
	return s, nil
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

// thread-safe session list
type SessionList struct {
	// WARNING: this mutex is created to prevent race conditions on certain
	// operations, like finding the most recent element of store, which are not
	// supported by golang-lru
	//
	// this is not really ideal since this introduces a chance of deadlocks.
	// please make sure that this mutex is never locked within the critical
	// section of the inner lock (belonging to store)
	mu    sync.RWMutex
	store *lru.Cache[string, *Session]
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
		// envFilesOrder = proj.EnvFilesReadOrder()
	}

	for _, specFile := range envSpecFiles {
		raw, _ := proj.LoadRawEnv(specFile)
		if raw == nil {
			continue
		}
		opt := owl.WithEnvFile(specFile, raw)
		for _, ssf := range envSpecFiles {
			if specFile == ssf {
				opt = owl.WithSpecFile(specFile, raw)
				break
			}
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
	return es.owlStore.InsecureGet(name)
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

func NewSessionList() (*SessionList, error) {
	// max integer
	capacity := int(^uint(0) >> 1)

	store, err := lru.New[string, *Session](capacity)
	if err != nil {
		return nil, err
	}

	return &SessionList{
		store: store,
	}, nil
}

func (sl *SessionList) AddSession(session *Session) {
	sl.store.Add(session.ID, session)
}

func (sl *SessionList) CreateAndAddSession(generate func() (*Session, error)) (*Session, error) {
	sess, err := generate()
	if err != nil {
		return nil, err
	}

	sl.AddSession(sess)
	return sess, nil
}

func (sl *SessionList) GetSession(id string) (*Session, bool) {
	return sl.store.Get(id)
}

func (sl *SessionList) DeleteSession(id string) (present bool) {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	sess, found := sl.GetSession(id)
	if found {
		sess.Complete()
	}

	return sl.store.Remove(id)
}

func (sl *SessionList) MostRecent() (*Session, bool) {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	return sl.mostRecentUnsafe()
}

func (sl *SessionList) mostRecentUnsafe() (*Session, bool) {
	keys := sl.store.Keys()

	sl.store.GetOldest()

	if len(keys) == 0 {
		return nil, false
	}

	return sl.store.Peek(keys[len(keys)-1])
}

func (sl *SessionList) MostRecentOrCreate(generate func() (*Session, error)) (*Session, error) {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	if existing, ok := sl.mostRecentUnsafe(); ok {
		return existing, nil
	}

	return sl.CreateAndAddSession(generate)
}

func (sl *SessionList) ListSessions() ([]*Session, error) {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	keys := sl.store.Keys()
	sessions := make([]*Session, len(keys))

	for i, k := range keys {
		sess, ok := sl.store.Peek(k)
		if !ok {
			return nil, fmt.Errorf("unexpected error: unable to find session %s when listing sessions from lru cache", sess.ID)
		}
		sessions[i] = sess
	}

	return sessions, nil
}

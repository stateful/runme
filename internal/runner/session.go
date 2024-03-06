package runner

import (
	"fmt"
	"sync"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/stateful/runme/v3/internal/owl"
	"github.com/stateful/runme/v3/internal/project"
	"github.com/stateful/runme/v3/internal/ulid"
	"go.uber.org/zap"
)

var owlStoreDefault = false

type envStorer interface {
	envs() []string
	addEnvs(envs []string) error
	updateStore(envs []string, newOrUpdated []string, deleted []string) error
	setEnv(k string, v string) error
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

func (s *Session) UpdateStore(envs []string, newOrUpdated []string, deleted []string) error {
	return s.envStorer.updateStore(envs, newOrUpdated, deleted)
}

func (s *Session) AddEnvs(envs []string) error {
	return s.envStorer.addEnvs(envs)
}

func (s *Session) SetEnv(k string, v string) error {
	return s.envStorer.setEnv(k, v)
}

func (s *Session) Envs() []string {
	vals := s.envStorer.envs()
	return vals
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
	logger   *zap.Logger
	envStore *envStore
}

func newRunnerStorer(sessionEnvs ...string) *runnerEnvStorer {
	return &runnerEnvStorer{
		envStore: newEnvStore(sessionEnvs...),
	}
}

func (es *runnerEnvStorer) addEnvs(envs []string) error {
	es.envStore.Add(envs...)
	return nil
}

func (es *runnerEnvStorer) envs() []string {
	envs, err := es.envStore.Values()
	if err != nil {
		es.logger.Error("failed to get envs", zap.Error(err))
		return nil
	}
	return envs
}

func (es *runnerEnvStorer) setEnv(k string, v string) error {
	_, err := es.envStore.Set(k, v)
	return err
}

func (es *runnerEnvStorer) updateStore(envs []string, newOrUpdated []string, deleted []string) error {
	es.envStore = newEnvStore(envs...).Add(newOrUpdated...).Delete(deleted...)
	return nil
}

type owlEnvStorer struct {
	logger   *zap.Logger
	owlStore *owl.Store
}

func newOwlStorer(envs []string, proj *project.Project, logger *zap.Logger) (*owlEnvStorer, error) {
	opts := []owl.StoreOption{
		owl.WithLogger(logger),
		owl.WithEnvs(envs...),
	}

	if proj != nil {
		specFilesOrder := proj.EnvFilesReadOrder()
		specFilesOrder = append([]string{".env.example"}, specFilesOrder...)
		for _, specFile := range specFilesOrder {
			raw, _ := proj.LoadRawEnv(specFile)
			if raw == nil {
				continue
			}
			opt := owl.WithEnvFile(specFile, raw)
			if specFile == ".env.example" {
				opt = owl.WithSpecFile(specFile, raw)
			}
			opts = append(opts, opt)
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

func (es *owlEnvStorer) updateStore(envs []string, newOrUpdated []string, deleted []string) error {
	return es.owlStore.Update(newOrUpdated, deleted)
}

func (es *owlEnvStorer) addEnvs(envs []string) error {
	return es.owlStore.Update(envs, nil)
}

func (es *owlEnvStorer) setEnv(k string, v string) error {
	// todo(sebastian): add checking env length inside Update
	err := es.owlStore.Update([]string{fmt.Sprintf("%s=%q", k, v)}, nil)
	if err != nil {
		return err
	}
	return err
}

func (es *owlEnvStorer) envs() []string {
	vals, err := es.owlStore.InsecureValues()
	if err != nil {
		es.logger.Error("failed to get vals", zap.Error(err))
		return nil
	}
	return vals
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

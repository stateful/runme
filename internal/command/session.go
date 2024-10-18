package command

import (
	"context"
	"fmt"

	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/stateful/runme/v3/internal/ulid"
	"github.com/stateful/runme/v3/pkg/project"
)

// Session is an object which lifespan contains multiple executions.
// It's used to exchange information between executions. Currently,
// it only keeps track of environment variables.
type Session struct {
	ID       string
	envStore EnvStore
}

type sessionFactory struct {
	owl     bool
	project *project.Project
	seedEnv []string
}

type SessionOption func(*sessionFactory) *sessionFactory

func WithOwl(owl bool) SessionOption {
	return func(f *sessionFactory) *sessionFactory {
		f.owl = owl
		return f
	}
}

func WithSessionProject(proj *project.Project) SessionOption {
	return func(f *sessionFactory) *sessionFactory {
		f.project = proj
		return f
	}
}

func WithSeedEnv(seedEnv []string) SessionOption {
	return func(f *sessionFactory) *sessionFactory {
		f.seedEnv = seedEnv
		return f
	}
}

// func NewSession(owl bool, proj *project.Project, seedEnv []string) (*Session, error) {
func NewSession(opts ...SessionOption) (*Session, error) {
	f := &sessionFactory{
		owl: false,
	}

	for _, opt := range opts {
		f = opt(f)
	}

	if !f.owl {
		return newSessionWithStore(newEnvStore(), f.project, f.seedEnv)
	}

	envStore, err := newOwlStore()
	if err != nil {
		return nil, err
	}

	return newSessionWithStore(envStore, f.project, f.seedEnv)
}

func newSessionWithStore(envStore EnvStore, proj *project.Project, seedEnv []string) (*Session, error) {
	sess := &Session{
		ID:       ulid.GenerateID(),
		envStore: envStore,
	}

	// seed session with system ENV vars
	if err := sess.envStore.Load("[system]", seedEnv...); err != nil {
		return nil, err
	}

	if err := sess.loadProject(proj); err != nil {
		return nil, err
	}

	return sess, nil
}

func (s *Session) SetEnv(ctx context.Context, env ...string) error {
	return s.envStore.Merge(ctx, env...)
}

func (s *Session) DeleteEnv(ctx context.Context, keys ...string) error {
	for _, k := range keys {
		if err := s.envStore.Delete(ctx, k); err != nil {
			return err
		}
	}
	return nil
}

func (s *Session) GetEnv(key string) (string, bool) {
	return s.envStore.Get(key)
}

func (s *Session) GetAllEnv() []string {
	if s == nil {
		return nil
	}
	items, _ := s.envStore.Items()
	return items
}

// loadProject loads from the project, it's not thread-safe.
func (s *Session) loadProject(proj *project.Project) error {
	if proj == nil {
		return nil
	}

	envWithSource, err := proj.LoadEnvWithSource()
	if err != nil {
		return err
	}

	for envSource, envMap := range envWithSource {
		envs := []string{}
		for k, v := range envMap {
			env := fmt.Sprintf("%s=%s", k, v)
			envs = append(envs, env)
		}
		if err := s.envStore.Load(envSource, envs...); err != nil {
			return err
		}
	}

	return nil
}

// SessionListCapacity is a maximum number of sessions
// stored in a single SessionList.
const SessionListCapacity = 1024

type SessionList struct {
	items *lru.Cache[string, *Session]
}

func NewSessionList() (*SessionList, error) {
	cache, err := lru.New[string, *Session](SessionListCapacity)
	if err != nil {
		return nil, err
	}
	return &SessionList{items: cache}, nil
}

func (sl *SessionList) Add(session *Session) {
	sl.items.Add(session.ID, session)
}

func (sl *SessionList) Get(id string) (*Session, bool) {
	return sl.items.Get(id)
}

func (sl *SessionList) List() []*Session {
	return sl.items.Values()
}

func (sl *SessionList) Delete(id string) bool {
	return sl.items.Remove(id)
}

func (sl *SessionList) Newest() (*Session, bool) {
	keys := sl.items.Keys()
	if len(keys) == 0 {
		return nil, false
	}
	return sl.items.Get(keys[len(keys)-1])
}

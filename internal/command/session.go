package command

import (
	"sync"

	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/stateful/runme/internal/ulid"
)

// Session is an object which lifespan contains multiple executions.
// It's used to exchange information between executions. Currently,
// it only keeps track of environment variables.
type Session struct {
	ID       string
	envStore *envStore
}

func NewSession() *Session {
	return &Session{
		ID:       ulid.GenerateID(),
		envStore: newEnvStore(),
	}
}

func NewSessionWithEnv(env ...string) (*Session, error) {
	s := NewSession()
	if err := s.SetEnv(env...); err != nil {
		return nil, err
	}
	return s, nil
}

func MustNewSessionWithEnv(env ...string) *Session {
	s, err := NewSessionWithEnv(env...)
	if err != nil {
		panic(err)
	}
	return s
}

func (s *Session) SetEnv(env ...string) error {
	_, err := s.envStore.Merge(env...)
	return err
}

func (s *Session) DeleteEnv(keys ...string) {
	for _, k := range keys {
		s.envStore.Delete(k)
	}
}

func (s *Session) GetEnv() []string {
	if s == nil {
		return nil
	}
	return s.envStore.Items()
}

// SessionListCapacity is a maximum number of sessions
// stored in a single SessionList.
const SessionListCapacity = 1024

type SessionList struct {
	items *lru.Cache[string, *Session]
	// Even though, the lry.Cache is thread-safe, it does not provide a way to
	// get the most recent added session.
	mu            sync.Mutex
	latestSession *Session
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

	sl.mu.Lock()
	sl.latestSession = session
	sl.mu.Unlock()
}

func (sl *SessionList) Get(id string) (*Session, bool) {
	return sl.items.Get(id)
}

func (sl *SessionList) List() []*Session {
	return sl.items.Values()
}

func (sl *SessionList) Delete(id string) bool {
	ok := sl.items.Remove(id)
	if !ok {
		return ok
	}

	sl.mu.Lock()
	if sl.latestSession != nil && sl.latestSession.ID == id {
		keys := sl.items.Keys()
		sl.latestSession, _ = sl.items.Get(keys[len(keys)-1])
	}
	sl.mu.Unlock()

	return ok
}

func (sl *SessionList) Newest() (*Session, bool) {
	return sl.latestSession, sl.latestSession != nil
}

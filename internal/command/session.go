package command

import (
	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/stateful/runme/v3/internal/ulid"
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

func (s *Session) SetEnv(env ...string) error {
	_, err := s.envStore.Merge(env...)
	return err
}

func (s *Session) DeleteEnv(keys ...string) {
	for _, k := range keys {
		s.envStore.Delete(k)
	}
}

func (s *Session) GetEnv(key string) (string, bool) {
	return s.envStore.Get(key)
}

func (s *Session) GetAllEnv() []string {
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

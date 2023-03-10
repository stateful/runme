package runner

import (
	"fmt"
	"sync"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/rs/xid"
	"go.uber.org/zap"
)

// Session is an abstract entity separate from
// an execution. Currently, its main role is to
// keep track of environment variables.
type Session struct {
	ID       string
	Metadata map[string]string

	envStore *envStore
	logger   *zap.Logger
}

func NewSession(envs []string, logger *zap.Logger) *Session {
	s := &Session{
		ID:       xid.New().String(),
		envStore: newEnvStore(envs...),
		logger:   logger,
	}
	return s
}

func (s *Session) AddEnvs(envs []string) {
	s.envStore.Add(envs...)
}

func (s *Session) Envs() []string {
	return s.envStore.Values()
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

func (sl *SessionList) CreateAndAddSession(generate func() *Session) *Session {
	sess := generate()
	sl.AddSession(sess)
	return sess
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

func (sl *SessionList) MostRecentOrCreate(generate func() *Session) *Session {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	if existing, ok := sl.mostRecentUnsafe(); ok {
		return existing
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

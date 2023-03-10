package runner

import (
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

type SessionList struct {
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
		store,
	}, nil
}

package runner

import (
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

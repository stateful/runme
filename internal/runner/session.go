package runner

import (
	"github.com/rs/xid"
	"go.uber.org/zap"
)

// session is an abstract entity separate from
// an execution. Currently, its main role is to
// keep track of environment variables.
type session struct {
	ID       string
	Metadata map[string]string

	envStore *envStore
	logger   *zap.Logger
}

func newSession(envs []string, logger *zap.Logger) *session {
	s := &session{
		ID:       xid.New().String(),
		envStore: newEnvStore(envs...),
		logger:   logger,
	}
	return s
}

func (s *session) Envs() []string {
	return s.envStore.Values()
}

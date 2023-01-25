package runner

import (
	"strings"

	"github.com/rs/xid"
	"go.uber.org/zap"
)

// session is an abstract entity separate from
// an execution. Currently, its main role is to
// keep track of environment variables.
type session struct {
	ID       string
	Metadata map[string]string
	envs     map[string]string
	logger   *zap.Logger
}

func newSession(envs []string, logger *zap.Logger) *session {
	s := &session{
		ID:     xid.New().String(),
		envs:   make(map[string]string),
		logger: logger,
	}
	s.AddEnvs(envs)
	return s
}

func (s *session) AddEnvs(envs []string) {
	for _, env := range envs {
		key, value := splitEnv(env)
		s.envs[key] = value
	}
}

func (s *session) Envs() (result []string) {
	for k, v := range s.envs {
		result = append(result, k+"="+v)
	}
	return
}

func splitEnv(str string) (string, string) {
	parts := strings.SplitN(str, "=", 2)
	switch len(parts) {
	case 0:
		return "", ""
	case 1:
		return parts[0], ""
	default:
		return parts[0], parts[1]
	}
}

package command

// Session is an object which lifespan contains multiple executions.
// It's used to exchange information between executions. Currently,
// it only keeps track of environment variables.
type Session struct {
	envStore *envStore
}

func NewSession() *Session {
	return &Session{
		envStore: newEnvStore(),
	}
}

func MustNewSessionWithEnv(env ...string) *Session {
	s := NewSession()
	if err := s.SetEnv(env...); err != nil {
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

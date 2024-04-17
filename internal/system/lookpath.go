package system

import (
	"os"
)

var Default = newDefault()

func newDefault() *System {
	return &System{
		getPathEnv: func() string { return os.Getenv("PATH") },
	}
}

type Option func(*System)

func WithPathEnvGetter(fn func() string) Option {
	return func(s *System) {
		s.getPathEnv = fn
	}
}

type System struct {
	getPathEnv func() string
}

func New(opts ...Option) *System {
	s := newDefault()

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *System) LookPath(file string) (string, error) {
	return lookPath(s.getPathEnv(), file)
}

package command

import (
	"strings"

	"github.com/pkg/errors"
)

const (
	MaxEnvSizeInBytes     = 128*1024 - 128
	MaxEnvironSizeInBytes = MaxEnvSizeInBytes * 8
)

var ErrEnvTooLarge = errors.New("env too large")

type EnvStore[T any] interface {
	Merge(envs ...string) (T, error)
	Get(k string) (string, bool)
	Set(k, v string) (T, error)
	Delete(k string) T
	Items() []string
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

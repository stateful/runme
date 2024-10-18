package session

import (
	"context"
	"strings"

	"github.com/pkg/errors"
)

const (
	MaxEnvSizeInBytes     = 128*1024 - 128
	MaxEnvironSizeInBytes = MaxEnvSizeInBytes * 8
)

var ErrEnvTooLarge = errors.New("env too large")

type EnvStore interface {
	Load(source string, envs ...string) error
	Merge(ctx context.Context, envs ...string) error
	Get(k string) (string, bool)
	Set(ctx context.Context, k, v string) error
	Delete(ctx context.Context, k string) error
	Items() ([]string, error)
}

func SplitEnv(str string) (string, string) {
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

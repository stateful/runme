package command

import (
	"os"

	"google.golang.org/protobuf/proto"
)

type envSource func() []string

type envNormalizer struct {
	sources []envSource
}

func (n *envNormalizer) Normalize(cfg *Config) (*Config, configNormalizerCleanupFunc, error) {
	result := proto.Clone(cfg).(*Config)

	env := os.Environ()
	env = append(env, cfg.Env...)

	for _, source := range n.sources {
		env = append(env, source()...)
	}

	result.Env = env

	return result, nil, nil
}

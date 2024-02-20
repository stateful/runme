package command

import (
	"os"

	"google.golang.org/protobuf/proto"
)

type envNormalizer struct {
	sources []func() []string
}

func newEnvNormalizer(sources ...func() []string) configNormalizer {
	return (&envNormalizer{sources: sources}).Normalize
}

func (n *envNormalizer) Normalize(cfg *Config) (*Config, func() error, error) {
	result := proto.Clone(cfg).(*Config)

	env := append(os.Environ(), cfg.Env...)

	for _, source := range n.sources {
		env = append(env, source()...)
	}

	result.Env = env

	return result, nil, nil
}

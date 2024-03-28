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

	// TODO: getting envs from OS should be configurable.
	result.Env = append(os.Environ(), cfg.Env...)

	for _, source := range n.sources {
		result.Env = append(result.Env, source()...)
	}

	return result, nil, nil
}

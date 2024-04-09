package command

type envNormalizer struct {
	kernel  Kernel
	sources []func() []string
}

func newEnvNormalizer(kernel Kernel, sources ...func() []string) configNormalizer {
	return (&envNormalizer{kernel: kernel, sources: sources}).Normalize
}

func (n *envNormalizer) Normalize(cfg *Config) (func() error, error) {
	// TODO: getting envs from OS should be configurable.
	cfg.Env = append(n.kernel.Environ(), cfg.Env...)

	for _, source := range n.sources {
		cfg.Env = append(cfg.Env, source()...)
	}

	return nil, nil
}

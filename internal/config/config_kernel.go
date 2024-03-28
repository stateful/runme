package config

type Kernel interface{}

type LocalKernel struct {
	// TODO(adamb): move config to LocalKernel
	// UseSystemEnv bool
	// EnvSources   []string
}

type DockerKernel struct {
	Image string
	Build struct {
		Context    string
		Dockerfile string
	}
}

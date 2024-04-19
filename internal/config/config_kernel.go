package config

type Kernel interface {
	GetName() string
}

type LocalKernel struct {
	// TODO(adamb): move config to LocalKernel
	// UseSystemEnv bool
	// EnvSources   []string
	Name string
}

func (k *LocalKernel) GetName() string {
	return k.Name
}

type DockerKernel struct {
	Build struct {
		Context    string
		Dockerfile string
	}
	Image string
	Name  string
}

func (k *DockerKernel) GetName() string {
	return k.Name
}

package command

import (
	"os"
	"os/exec"

	"github.com/stateful/runme/v3/internal/config"
	"github.com/stateful/runme/v3/internal/dockerexec"
	"go.uber.org/zap"
)

type Kernel interface {
	Command(*Config, Options) Command
	Environ() []string
	LookPath(string) (string, error)
}

type LocalKernel struct{}

func NewLocalKernel(cfg *config.LocalKernel) *LocalKernel {
	return &LocalKernel{}
}

func (k *LocalKernel) Command(cfg *Config, opts Options) Command {
	if cfg.Interactive {
		return NewVirtual(cfg, opts)
	}
	return NewNative(cfg, opts)
}

func (k *LocalKernel) Environ() []string {
	return os.Environ()
}

func (k *LocalKernel) LookPath(path string) (string, error) {
	return exec.LookPath(path)
}

type DockerKernel struct {
	docker *dockerexec.Docker
}

func NewDockerKernel(cfg *config.DockerKernel, logger *zap.Logger) (*DockerKernel, error) {
	d, err := dockerexec.New(&dockerexec.Options{
		Image:        cfg.Image,
		BuildContext: cfg.Build.Context,
		Dockerfile:   cfg.Build.Dockerfile,
		Logger:       logger,
	})
	if err != nil {
		return nil, err
	}
	return &DockerKernel{docker: d}, nil
}

func (k *DockerKernel) Command(cfg *Config, opts Options) Command {
	return NewDocker(cfg, k.docker, opts)
}

func (k *DockerKernel) Environ() []string {
	return nil
}

func (k *DockerKernel) LookPath(path string) (string, error) {
	// TODO(adamb): implement
	return path, nil
}

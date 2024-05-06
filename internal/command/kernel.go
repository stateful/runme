package command

import (
	"os"
	"os/exec"

	"github.com/stateful/runme/v3/internal/config"
	"github.com/stateful/runme/v3/internal/dockerexec"
)

// Kernel represents an execution environment for commands.
// It abstracts all OS-specific details and provides a unified interface.
type Kernel interface {
	Environ() []string
	GetEnv(string) string
	LookPath(string) (string, error)
}

type LocalKernel struct{}

func NewLocalKernel(cfg *config.LocalKernel) *LocalKernel {
	return &LocalKernel{}
}

func (k *LocalKernel) Environ() []string {
	return os.Environ()
}

func (k *LocalKernel) GetEnv(key string) string {
	return os.Getenv(key)
}

func (k *LocalKernel) LookPath(path string) (string, error) {
	return exec.LookPath(path)
}

type DockerKernel struct {
	docker *dockerexec.Docker
}

func NewDockerKernel(docker *dockerexec.Docker) *DockerKernel {
	return &DockerKernel{docker: docker}
}

func (k *DockerKernel) Environ() []string {
	// TODO(adamb): implement
	return nil
}

func (k *DockerKernel) GetEnv(key string) string {
	// TODO(adamb): implement
	return ""
}

func (k *DockerKernel) LookPath(path string) (string, error) {
	// TODO(adamb): implement
	return path, nil
}

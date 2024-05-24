package command

import (
	"os"
	"os/exec"

	"github.com/stateful/runme/v3/internal/dockerexec"
)

// Runtime represents an execution environment for commands.
// It abstracts all OS-specific details and provides a unified interface.
type Runtime interface {
	Environ() []string
	GetEnv(string) string
	LookPath(string) (string, error)
}

type Host struct{}

var _ Runtime = (*Host)(nil)

func NewHostRuntime() *Host {
	return &Host{}
}

func (Host) Environ() []string {
	return os.Environ()
}

func (Host) GetEnv(key string) string {
	return os.Getenv(key)
}

func (Host) LookPath(path string) (string, error) {
	return exec.LookPath(path)
}

type Docker struct {
	docker *dockerexec.Docker
}

var _ Runtime = (*Docker)(nil)

func NewDockerRuntime(docker *dockerexec.Docker) *Docker {
	return &Docker{docker: docker}
}

func (Docker) Environ() []string {
	// TODO(adamb): implement
	return nil
}

func (Docker) GetEnv(key string) string {
	// TODO(adamb): implement
	return ""
}

func (Docker) LookPath(path string) (string, error) {
	// TODO(adamb): implement
	return path, nil
}

package command

import (
	"os"

	"github.com/stateful/runme/v3/internal/dockerexec"
	"github.com/stateful/runme/v3/internal/system"
)

type runtime interface {
	Environ() []string
	LookPathUsingPathEnv(file, pathEnv string) (string, error)
}

type hostRuntime struct{}

func (hostRuntime) Environ() []string {
	return os.Environ()
}

func (hostRuntime) LookPathUsingPathEnv(file, pathEnv string) (string, error) {
	return system.LookPathUsingPathEnv(file, pathEnv)
}

type dockerRuntime struct {
	*dockerexec.Docker
}

func (dockerRuntime) Environ() []string {
	return nil
}

func (dockerRuntime) LookPathUsingPathEnv(file, _ string) (string, error) {
	return file, nil
}

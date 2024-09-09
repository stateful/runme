package command

import (
	"os"

	"github.com/stateful/runme/v3/internal/dockerexec"
	"github.com/stateful/runme/v3/internal/system"
)

type Runtime interface {
	Environ() []string
	LookPathUsingPathEnv(file, pathEnv string) (string, error)
}

type hostRuntime struct {
	fixedEnviron []string
	useSystem    bool
}

func (r hostRuntime) Environ() []string {
	result := make([]string, len(r.fixedEnviron))
	copy(result, r.fixedEnviron)

	if r.useSystem {
		systemEnv := os.Environ()
		result = append(result, systemEnv...)
	}

	return result
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

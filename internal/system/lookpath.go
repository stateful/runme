package system

import (
	"os/exec"
)

func LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

func LookPathUsingPathEnv(file, pathEnv string) (string, error) {
	if pathEnv == "" {
		return exec.LookPath(file)
	}
	return lookPath(pathEnv, file)
}

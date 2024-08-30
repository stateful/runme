package beta

import (
	"os"
)

func getCwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	return cwd
}

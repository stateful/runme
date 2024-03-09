package beta

import (
	"os"
	"path/filepath"
)

var cwd = getCwd()

func getCwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	return cwd
}

func relativePathToCwd(path string) string {
	relPath, err := filepath.Rel(cwd, path)
	if err != nil {
		relPath = path
	}
	return relPath
}

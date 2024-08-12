//go:build !windows

package main

import (
	"bufio"
	"os"
	"strings"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

func isDocker() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	paths := []string{"/proc/1/cgroup", "/proc/self/cgroup"}
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(file)
		isDocker := false
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), "docker") || strings.Contains(scanner.Text(), "kubepods") {
				isDocker = true
				break
			}
		}

		if err := scanner.Err(); err != nil {
			_ = file.Close()
			return false
		}

		_ = file.Close()

		if isDocker {
			return true
		}
	}

	return false
}

func TestRunmeFilePermissions(t *testing.T) {
	if isDocker() {
		t.Skip("Test skipped when running inside a Docker container")
		return
	}

	testscript.Run(t, testscript.Params{
		Dir:             "testdata/permissions",
		ContinueOnError: true,
	})
}

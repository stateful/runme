//go:build !windows

package main

import (
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
	"github.com/stateful/runme/v3/internal/testutils"
)

func TestRunmeFilePermissions(t *testing.T) {
	if testutils.IsRunningInDocker() {
		t.Skip("Test skipped when running inside a Docker container")
	}

	testscript.Run(t, testscript.Params{
		Dir:             "testdata/permissions",
		ContinueOnError: true,
	})
}

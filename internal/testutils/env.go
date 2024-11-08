package testutils

import "os"

// IsDockerTestEnv returns true if the test is running in a Docker environment.
// Check out the Makefile's "test-docker/run" target where "RUNME_TEST_ENV"
// is set to "docker".
func IsDockerTestEnv() bool {
	return os.Getenv("RUNME_TEST_ENV") == "docker"
}

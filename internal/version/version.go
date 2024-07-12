package version

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
)

var (
	BuildDate    = "unknown"
	BuildVersion = "0.0.0"
	Commit       = "unknown"
)

func BaseVersionInfo() string {
	return fmt.Sprintf("%s (%s) on %s", BuildVersion, Commit, BuildDate)
}

// BaseVersion returns the base version of the application.
func BaseVersion() string {
	v, err := semver.NewVersion(BuildVersion)
	if err != nil {
		return BuildDate
	}

	return fmt.Sprintf("v%d", v.Major())
}

func BaseVersionAuthoritative() (string, bool) {
	_, err := semver.NewVersion(BuildVersion)
	baseVersion := BaseVersion()
	return baseVersion, err == nil && baseVersion != "v0" && baseVersion != "v99"
}

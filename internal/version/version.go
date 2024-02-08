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

// BaseVersion returns the base version of the application.
func BaseVersion() string {
	v, err := semver.NewVersion(BuildVersion)
	if err != nil {
		return BuildDate
	}

	return fmt.Sprintf("v%d.%d", v.Major(), v.Minor())
}

func BaseVersionAuthoritative() (string, bool) {
	_, err := semver.NewVersion(BuildVersion)
	baseVersion := BaseVersion()
	return baseVersion, err == nil && baseVersion != "v0.0" && baseVersion != "v99.9"
}

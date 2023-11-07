package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBaseVersion(t *testing.T) {
	tests := []struct {
		name          string
		buildVersion  string
		expectedMinor string
	}{
		{
			name:          "standard version",
			buildVersion:  "1.7.8-11-g2300850-2300850",
			expectedMinor: "1.7",
		},
		{
			name:          "only major and minor version",
			buildVersion:  "2.3-11-g2300850-2300850",
			expectedMinor: "2.3",
		},
		{
			name:          "only major version",
			buildVersion:  "3-11-g2300850-2300850",
			expectedMinor: "3.0",
		},
		{
			name:          "no version",
			buildVersion:  "0.0.0",
			expectedMinor: "0.0",
		},
		{
			name:          "invalid semver",
			buildVersion:  "1.2.beta",
			expectedMinor: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			BuildVersion = tt.buildVersion
			baseVersion := BaseVersion()
			assert.Equal(t, tt.expectedMinor, baseVersion)
		})
	}
}

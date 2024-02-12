package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBaseVersion(t *testing.T) {
	tests := []struct {
		name                string
		buildVersion        string
		expectedBaseVersion string
	}{
		{
			name:                "standard version",
			buildVersion:        "1.7.8-11-g2300850-2300850",
			expectedBaseVersion: "v1",
		},
		{
			name:                "only major and minor version",
			buildVersion:        "2.3-11-g2300850-2300850",
			expectedBaseVersion: "v2",
		},
		{
			name:                "only major version",
			buildVersion:        "3-11-g2300850-2300850",
			expectedBaseVersion: "v3",
		},
		{
			name:                "no version",
			buildVersion:        "0.0.0",
			expectedBaseVersion: "v0",
		},
		{
			name:                "invalid semver",
			buildVersion:        "1.2.beta",
			expectedBaseVersion: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			BuildVersion = tt.buildVersion
			baseVersion := BaseVersion()
			assert.Equal(t, tt.expectedBaseVersion, baseVersion)
		})
	}
}

func TestBaseVersionAuthoritative(t *testing.T) {
	tests := []struct {
		name                string
		buildVersion        string
		expectedBaseVersion string
		authoritative       bool
	}{
		{
			name:                "zeros",
			buildVersion:        "0.0.0",
			expectedBaseVersion: "v0",
		},
		{
			name:                "nines",
			buildVersion:        "99.9.9",
			expectedBaseVersion: "v99",
		},
		{
			name:                "standard version",
			buildVersion:        "1.7.8-11-g2300850-2300850",
			expectedBaseVersion: "v1",
			authoritative:       true,
		},
		{
			name:                "invalid semver",
			buildVersion:        "1.2.beta",
			expectedBaseVersion: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			BuildVersion = tt.buildVersion
			baseVersion, authoritative := BaseVersionAuthoritative()
			assert.Equal(t, tt.expectedBaseVersion, baseVersion)
			assert.Equal(t, tt.authoritative, authoritative)
		})
	}
}

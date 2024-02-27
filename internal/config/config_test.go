package config

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
)

func TestParseYAML(t *testing.T) {
	testCases := []struct {
		name           string
		rawConfig      string
		expectedConfig *Config
		errorSubstring string
	}{
		{
			name:           "full config v1alpha1",
			rawConfig:      testConfigV1alpha1Raw,
			expectedConfig: testConfigV1alpha1,
		},
		{
			name:           "minimal config v1alpha1",
			rawConfig:      "version: v1alpha1\nfilename: REAEDME.md\n",
			expectedConfig: &Config{Filename: "REAEDME.md"},
		},
		{
			name:           "validate source",
			rawConfig:      "version: v1alpha1",
			errorSubstring: "failed to validate v1alpha1 config: validation error:\n - source: exactly one field is required",
		},
		{
			name:           "validate filter type",
			rawConfig:      "version: v1alpha1\nfilename: README.md\nfilters:\n  - type: 3\n    condition: \"name != ''\"\n",
			errorSubstring: "failed to validate v1alpha1 config: validation error:\n - filters[0].type: value must be one of the defined enum values",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config, err := ParseYAML([]byte(tc.rawConfig))

			if tc.errorSubstring != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errorSubstring)
				return
			}

			require.NoError(t, err)
			require.True(
				t,
				cmp.Equal(
					tc.expectedConfig,
					config,
					cmpopts.IgnoreUnexported(Filter{}),
				),
				"%s", cmp.Diff(tc.expectedConfig, config, cmpopts.IgnoreUnexported(Filter{})),
			)
		})
	}
}

var (
	testConfigV1alpha1Raw = `version: v1alpha1

project:
  dir: "."
  find_repo_upward: true
  ignore:
    - "node_modules"
    - ".venv"
  disable_gitignore: false

env:
  sources:
    - ".env"

filters:
  - type: "FILTER_TYPE_BLOCK"
    condition: "name != ''"

log:
  enabled: true
  path: "/var/tmp/runme.log"
  verbose: true
`

	testConfigV1alpha1 = &Config{
		ProjectDir:       ".",
		FindRepoUpward:   true,
		IgnorePaths:      []string{"node_modules", ".venv"},
		DisableGitignore: false,

		EnvSourceFiles: []string{".env"},

		Filters: []*Filter{
			{
				Type:      "FILTER_TYPE_BLOCK",
				Condition: "name != ''",
			},
		},

		LogEnabled: true,
		LogPath:    "/var/tmp/runme.log",
		LogVerbose: true,
	}
)

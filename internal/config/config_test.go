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
		// TODO(adamb): test validation
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config, err := ParseYAML([]byte(tc.rawConfig))
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
  - ".env"

chdir: "."

filters:
  - type: "FILTER_TYPE_BLOCK"
    condition: "name != ''"

server:
  address: "localhost:7863"
  insecure: false

log:
  enable: true
  path: "/var/tmp/runme.log"
  verbose: true
`

	testConfigV1alpha1 = &Config{
		ProjectDir:       ".",
		FindRepoUpward:   true,
		IgnorePaths:      []string{"node_modules", ".venv"},
		DisableGitignore: false,

		EnvPaths: []string{".env"},

		Chdir: ".",

		Filters: []*Filter{
			{
				Type:      "FILTER_TYPE_BLOCK",
				Condition: "name != ''",
			},
		},

		ServerAddr: "localhost:7863",

		LogEnable:  true,
		LogPath:    "/var/tmp/runme.log",
		LogVerbose: true,
	}
)

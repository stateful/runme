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
			rawConfig:      "version: v1alpha1\nproject:\n  filename: REAEDME.md\n",
			expectedConfig: &Config{Kernel: ConfigKernel{Filename: "REAEDME.md"}},
		},
		{
			name:           "validate source",
			rawConfig:      "version: v1alpha1",
			expectedConfig: &Config{},
		},
		{
			name:           "project and filename",
			rawConfig:      "version: v1alpha1\nproject:\n  root: \".\"\n  filename: \"README.md\"\n",
			errorSubstring: "error parsing \"root\", oneof runme.config.v1alpha1.Project.source is already set",
		},
		{
			name:           "validate filter type",
			rawConfig:      "version: v1alpha1\nproject:\n  filename: README.md\n  filters:\n  - type: 3\n    condition: \"name != ''\"\n",
			errorSubstring: "failed to validate v1alpha1 config: validation error:\n - project.filters[0].type: value must be one of the defined enum values",
		},
		{
			name:           "validate project within cwd",
			rawConfig:      "version: v1alpha1\nproject:\n  root: '..'\n",
			errorSubstring: "failed to validate config: failed to validate project dir: outside of current working directory",
		},
		{
			name:           "validate filename within cwd",
			rawConfig:      "version: v1alpha1\nproject:\n  filename: '../README.md'\n",
			errorSubstring: "failed to validate config: failed to validate filename: outside of current working directory",
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

kernel:
  log:
    enabled: true
    path: "/var/tmp/runme.log"
    verbose: true

project:
  root: "."
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
`

	testConfigV1alpha1 = &Config{
		Kernel: ConfigKernel{
			ProjectDir:       ".",
			FindRepoUpward:   true,
			IgnorePaths:      []string{"node_modules", ".venv"},
			DisableGitignore: false,

			EnvSourceFiles: []string{".env"},

			LogEnabled: true,
			LogPath:    "/var/tmp/runme.log",
			LogVerbose: true,
		},
		Repo: ConfigRepo{
			Filters: []*Filter{
				{
					Type:      "FILTER_TYPE_BLOCK",
					Condition: "name != ''",
				},
			},
		},
	}
)

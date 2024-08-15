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
			name: "only filename",
			rawConfig: `version: v1alpha1
project:
  filename: REAEDME.md
`,
			expectedConfig: &Config{ProjectFilename: "REAEDME.md", ProjectFindRepoUpward: true, ServerTLSEnabled: true},
		},
		{
			name: "root and filename",
			rawConfig: `version: v1alpha1
project:
  root: "."
  filename: README.md
`,
			expectedConfig: &Config{ProjectRoot: ".", ProjectFilename: "README.md", ProjectFindRepoUpward: true, ServerTLSEnabled: true},
		},
		{
			name: "validate filter type",
			rawConfig: `version: v1alpha1
project:
  filename: README.md
  filters:
    - type: 3
      condition: "name != ''"
`,
			errorSubstring: "failed to validate v1alpha1 config: validation error:\n - project.filters[0].type: value must be one of the defined enum values",
		},
		{
			name: "validate project within cwd",
			rawConfig: `version: v1alpha1
project:
  root: '..'
`,
			errorSubstring: "failed to validate config: failed to validate project dir: outside of the current working directory",
		},
		{
			name: "validate filename within cwd",
			rawConfig: `version: v1alpha1
project:
  filename: '../README.md'
`,
			errorSubstring: "failed to validate config: failed to validate filename: outside of the current working directory",
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

runtime:
  docker:
    enabled: false
    image: "runme-runner:latest"
    build:
      context: "./experimental/docker"
      dockerfile: "Dockerfile"

log:
  enabled: true
  path: "/var/tmp/runme.log"
  verbose: true
`

	testConfigV1alpha1 = &Config{
		ProjectRoot:             ".",
		ProjectFindRepoUpward:   true,
		ProjectIgnorePaths:      []string{"node_modules", ".venv"},
		ProjectDisableGitignore: false,
		ProjectEnvSources:       []string{".env"},
		ProjectFilters: []*Filter{
			{
				Type:      "FILTER_TYPE_BLOCK",
				Condition: "name != ''",
			},
		},

		RuntimeDockerEnabled:         false,
		RuntimeDockerImage:           "runme-runner:latest",
		RuntimeDockerBuildContext:    "./experimental/docker",
		RuntimeDockerBuildDockerfile: "Dockerfile",

		LogEnabled: true,
		LogPath:    "/var/tmp/runme.log",
		LogVerbose: true,

		ServerTLSEnabled: true,
	}
)

func TestCloneConfig(t *testing.T) {
	original := defaults
	clone := original.Clone()

	opts := cmpopts.EquateEmpty()
	require.True(
		t,
		cmp.Equal(&original, clone, opts),
		"%s",
		cmp.Diff(&original, clone, opts),
	)
	require.False(t, &original == clone)
	require.False(t, &original.ProjectIgnorePaths == &clone.ProjectIgnorePaths)
	require.False(t, &original.ProjectEnvSources == &clone.ProjectEnvSources)
	require.False(t, &original.ProjectFilters == &clone.ProjectFilters)
}

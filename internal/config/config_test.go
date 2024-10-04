package config

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
)

func TestDefault(t *testing.T) {
	// Invariant that all default configurations are equal.
	expected, err := newDefault()
	require.NoError(t, err)
	got := Default()
	opts := cmpopts.EquateEmpty()
	require.True(
		t,
		cmp.Equal(expected, got, opts),
		"%s",
		cmp.Diff(expected, got, opts),
	)
}

func Test_parseYAML(t *testing.T) {
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
			expectedConfig: &Config{
				Project: ConfigProject{Filename: "REAEDME.md"},
				Version: "v1alpha1",
			},
		},
		{
			name: "root and filename",
			rawConfig: `version: v1alpha1
project:
  root: "."
  filename: README.md
`,
			expectedConfig: &Config{
				Version: "v1alpha1",
				Project: ConfigProject{Root: ".", Filename: "README.md"},
			},
		},
		{
			name: "validate filter type",
			rawConfig: `version: v1alpha1
project:
  filename: README.md
  filters:
    - type: "FILTER_TYPE_SOME_OTHER"
      condition: "name != ''"
`,
			errorSubstring: "failed to parse v1alpha1 config: invalid value (expected one of []interface {}{\"FILTER_TYPE_BLOCK\", \"FILTER_TYPE_DOCUMENT\"}): \"FILTER_TYPE_SOME_OTHER\"",
		},
		{
			name: "validate project within cwd",
			rawConfig: `version: v1alpha1
project:
  root: '..'
`,
			errorSubstring: "failed to validate v1alpha1 config: project.root: outside of the current working directory",
		},
		{
			name: "validate filename within cwd",
			rawConfig: `version: v1alpha1
project:
  filename: '../README.md'
`,
			errorSubstring: "failed to validate v1alpha1 config: project.filename: outside of the current working directory",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config, err := parseYAML([]byte(tc.rawConfig))

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

func Test_parseYAML_Multiple(t *testing.T) {
	cfg1 := []byte(`version: v1alpha1`)
	cfg2 := []byte(`version: v1alpha1
project:
  filename: README.md
`)
	expected := &Config{
		Version: "v1alpha1",
		Project: ConfigProject{
			Filename: "README.md",
		},
	}

	config, err := parseYAML(cfg1, cfg2)
	require.NoError(t, err)
	require.True(
		t,
		cmp.Equal(
			expected,
			config,
			cmpopts.IgnoreUnexported(Filter{}),
		),
		"%s", cmp.Diff(expected, config, cmpopts.IgnoreUnexported(Filter{})),
	)
}

func TestParseYAML(t *testing.T) {
	got, err := ParseYAML([]byte(testConfigV1alpha1Raw))
	require.NoError(t, err)
	expected := *testConfigV1alpha1
	expected.Server = &ConfigServer{
		Address: "localhost:7998",
		Tls: &ConfigServerTls{
			Enabled: true,
		},
	}
	require.True(
		t,
		cmp.Equal(
			&expected,
			got,
			cmpopts.IgnoreUnexported(Filter{}),
		),
		"%s", cmp.Diff(&expected, got, cmpopts.IgnoreUnexported(Filter{})),
	)
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
		Version: "v1alpha1",

		Project: ConfigProject{
			Root:             ".",
			FindRepoUpward:   true,
			Ignore:           []string{"node_modules", ".venv"},
			DisableGitignore: false,
			Env: &ConfigProjectEnv{
				Sources: []string{".env"},
			},
			Filters: []ConfigProjectFiltersElem{
				{
					Type:      ConfigProjectFiltersElemTypeFILTERTYPEBLOCK,
					Condition: "name != ''",
				},
			},
		},

		Runtime: &ConfigRuntime{
			Docker: &ConfigRuntimeDocker{
				Enabled: false,
				Image:   "runme-runner:latest",
				Build: &ConfigRuntimeDockerBuild{
					Context:    "./experimental/docker",
					Dockerfile: "Dockerfile",
				},
			},
		},

		Log: &ConfigLog{
			Enabled: true,
			Path:    "/var/tmp/runme.log",
			Verbose: true,
		},
	}
)

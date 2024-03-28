package config

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/anypb"

	configv1alpha1 "github.com/stateful/runme/v3/internal/gen/proto/go/runme/config/v1alpha1"
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
			name:           "project and filename",
			rawConfig:      "version: v1alpha1\nproject:\n  dir: \".\"\nfilename: \"README.md\"\n",
			errorSubstring: "error parsing \"project\", oneof runme.config.v1alpha1.Config.source is already set",
		},
		{
			name:           "validate filter type",
			rawConfig:      "version: v1alpha1\nfilename: README.md\nfilters:\n  - type: 3\n    condition: \"name != ''\"\n",
			errorSubstring: "failed to validate v1alpha1 config: validation error:\n - filters[0].type: value must be one of the defined enum values",
		},
		{
			name:           "validate project within cwd",
			rawConfig:      "version: v1alpha1\nproject:\n  dir: '..'\n",
			errorSubstring: "failed to validate config: failed to validate project dir: outside of current working directory",
		},
		{
			name:           "validate filename within cwd",
			rawConfig:      "version: v1alpha1\nfilename: '../README.md'\n",
			errorSubstring: "failed to validate config: failed to validate filename: outside of current working directory",
		},
		{
			name:           "invalid kernel type",
			rawConfig:      "version: v1alpha1\nfilename: REAEDME.md\nkernels:\n  - '@type': 'type.googleapis.com/runme.config.v1alpha1.Config.Unknown'\n",
			errorSubstring: "unable to resolve \"type.googleapis.com/runme.config.v1alpha1.Config.Unknown\": \"not found\"",
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

kernels:
  - "@type": "type.googleapis.com/runme.config.v1alpha1.Config.DockerKernel"
    image: runme-kernel:latest
    build:
      context: ./experimental/kernel
      dockerfile: Dockerfile
  - "@type": "type.googleapis.com/runme.config.v1alpha1.Config.LocalKernel"

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

		Kernels: []Kernel{
			&DockerKernel{
				Image: "runme-kernel:latest",
				Build: struct {
					Context    string
					Dockerfile string
				}{
					Context:    "./experimental/kernel",
					Dockerfile: "Dockerfile",
				},
			},
			&LocalKernel{},
		},

		LogEnabled: true,
		LogPath:    "/var/tmp/runme.log",
		LogVerbose: true,
	}
)

func TestKernelProtoJSONToProto(t *testing.T) {
	localKernel, err := anypb.New(&configv1alpha1.Config_LocalKernel{})
	require.NoError(t, err)

	obj := &configv1alpha1.Config{
		Kernels: []*anypb.Any{
			localKernel,
		},
	}

	data, err := protojson.Marshal(obj)
	require.NoError(t, err)
	require.Equal(t, `{"kernels":[{"@type":"type.googleapis.com/runme.config.v1alpha1.Config.LocalKernel"}]}`, string(data))
}

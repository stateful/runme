package command

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	runnerv2alpha1 "github.com/stateful/runme/internal/gen/proto/go/runme/runner/v2alpha1"
)

func TestEnvResolver_Parsing(t *testing.T) {
	createResultWithUnresolvedEnv := func(name string, originalValue string) *ResolveEnvResult {
		return &ResolveEnvResult{
			Result: &runnerv2alpha1.ResolveEnvResult_UnresolvedEnv_{
				UnresolvedEnv: &runnerv2alpha1.ResolveEnvResult_UnresolvedEnv{
					Name:          name,
					OriginalValue: originalValue,
				},
			},
		}
	}

	testCases := []struct {
		name   string
		data   string
		source []EnvResolverSource
		result []*ResolveEnvResult
	}{
		{
			name: "no value",
			data: `export TEST_NO_VALUE`,
			result: []*ResolveEnvResult{
				createResultWithUnresolvedEnv("TEST_NO_VALUE", ""),
			},
		},
		{
			name: "empty value",
			data: `export TEST_EMPTY_VALUE=`,
			result: []*ResolveEnvResult{
				createResultWithUnresolvedEnv("TEST_EMPTY_VALUE", ""),
			},
		},
		{
			name: "string value",
			data: `export TEST_STRING_VALUE=value`,
			result: []*ResolveEnvResult{
				createResultWithUnresolvedEnv("TEST_STRING_VALUE", "value"),
			},
		},
		{
			name: "string double quoted value empty",
			data: `export TEST_STRING_DBL_QUOTED_VALUE_EMPTY=""`,
			result: []*ResolveEnvResult{
				createResultWithUnresolvedEnv("TEST_STRING_DBL_QUOTED_VALUE_EMPTY", ""),
			},
		},
		{
			name: "string double quoted value",
			data: `export TEST_STRING_DBL_QUOTED_VALUE="value"`,
			result: []*ResolveEnvResult{
				createResultWithUnresolvedEnv("TEST_STRING_DBL_QUOTED_VALUE", "value"),
			},
		},
		{
			name: "string single quoted value empty",
			data: `export TEST_STRING_SGL_QUOTED_VALUE_EMPTY=''`,
			result: []*ResolveEnvResult{
				createResultWithUnresolvedEnv("TEST_STRING_SGL_QUOTED_VALUE_EMPTY", ""),
			},
		},
		{
			name: "string single quoted value",
			data: `export TEST_STRING_SGL_QUOTED_VALUE='value'`,
			result: []*ResolveEnvResult{
				createResultWithUnresolvedEnv("TEST_STRING_SGL_QUOTED_VALUE", "value"),
			},
		},
		{
			name: "value expression",
			data: `export TEST_VALUE_EXPR=$(echo -n "value")`,
			result: []*ResolveEnvResult{
				createResultWithUnresolvedEnv("TEST_VALUE_EXPR", ""),
			},
		},
		{
			name: "default value",
			data: `export TEST_DEFAULT_VALUE=${TEST_DEFAULT_VALUE:-value}`,
			result: []*ResolveEnvResult{
				createResultWithUnresolvedEnv("TEST_DEFAULT_VALUE", "value"),
			},
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			r := NewEnvResolver(tc.source...)
			result, err := r.Resolve(strings.NewReader(tc.data))
			assert.NoError(t, err)
			assert.EqualValues(t, tc.result, result)
		})
	}
}

func TestEnvResolver(t *testing.T) {
	createResultResolvedEnv := func(name, resolvedValue, originalValue string) *ResolveEnvResult {
		return &ResolveEnvResult{
			Result: &runnerv2alpha1.ResolveEnvResult_ResolvedEnv_{
				ResolvedEnv: &runnerv2alpha1.ResolveEnvResult_ResolvedEnv{
					Name:          name,
					ResolvedValue: resolvedValue,
					OriginalValue: originalValue,
				},
			},
		}
	}

	r := NewEnvResolver(EnvResolverSourceFunc([]string{"MY_ENV=resolved"}))
	result, err := r.Resolve(strings.NewReader(`export MY_ENV=default`))
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.EqualValues(t, createResultResolvedEnv("MY_ENV", "resolved", "default"), result[0])
}

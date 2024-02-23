package command

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvResolverParsing(t *testing.T) {
	testCases := []struct {
		name   string
		data   string
		source []EnvResolverSource
		result []*EnvResolverResult
	}{
		{
			name: "no value",
			data: `export TEST_NO_VALUE`,
			result: []*EnvResolverResult{
				{Name: "TEST_NO_VALUE", Prompt: EnvResolverMessage},
			},
		},
		{
			name: "empty value",
			data: `export TEST_EMPTY_VALUE=`,
			result: []*EnvResolverResult{
				{Name: "TEST_EMPTY_VALUE", Prompt: EnvResolverMessage},
			},
		},
		{
			name: "string value",
			data: `export TEST_STRING_VALUE=value`,
			result: []*EnvResolverResult{
				{Name: "TEST_STRING_VALUE", OriginalValue: "value", Prompt: EnvResolverMessage},
			},
		},
		{
			name: "string value with equal sign",
			data: `export TEST_STRING_VALUE_WITH_EQUAL_SIGN=part1=part2`,
			result: []*EnvResolverResult{
				{Name: "TEST_STRING_VALUE_WITH_EQUAL_SIGN", OriginalValue: "part1=part2", Prompt: EnvResolverMessage},
			},
		},
		{
			name: "string double quoted value empty",
			data: `export TEST_STRING_DBL_QUOTED_VALUE_EMPTY=""`,
			result: []*EnvResolverResult{
				{Name: "TEST_STRING_DBL_QUOTED_VALUE_EMPTY", OriginalValue: "", Prompt: EnvResolverMessage},
			},
		},
		{
			name: "string double quoted value",
			data: `export TEST_STRING_DBL_QUOTED_VALUE="value"`,
			result: []*EnvResolverResult{
				{Name: "TEST_STRING_DBL_QUOTED_VALUE", OriginalValue: "value", Prompt: EnvResolverPlaceholder},
			},
		},
		{
			name: "string single quoted value empty",
			data: `export TEST_STRING_SGL_QUOTED_VALUE_EMPTY=''`,
			result: []*EnvResolverResult{
				{Name: "TEST_STRING_SGL_QUOTED_VALUE_EMPTY", OriginalValue: "", Prompt: EnvResolverPlaceholder},
			},
		},
		{
			name: "string single quoted value",
			data: `export TEST_STRING_SGL_QUOTED_VALUE='value'`,
			result: []*EnvResolverResult{
				{Name: "TEST_STRING_SGL_QUOTED_VALUE", OriginalValue: "value", Prompt: EnvResolverPlaceholder},
			},
		},
		{
			name:   "value expression",
			data:   `export TEST_VALUE_EXPR=$(echo -n "value")`,
			result: nil,
		},
		{
			name:   "double quoted value expression",
			data:   `export TEST_DBL_QUOTE_VALUE_EXPR="$(echo -n 'value')"`,
			result: nil,
		},
		{
			name:   "default value",
			data:   `export TEST_DEFAULT_VALUE=${TEST_DEFAULT_VALUE:-value}`,
			result: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := NewEnvResolver(EnvResolverModeAuto, tc.source...)
			var script bytes.Buffer
			result, err := r.Resolve(strings.NewReader(tc.data), &script)
			assert.NoError(t, err)
			assert.EqualValues(t, tc.result, result)
		})
	}
}

func TestEnvResolverResolve(t *testing.T) {
	r := NewEnvResolver(EnvResolverModeAuto, EnvResolverSourceFunc([]string{"MY_ENV=resolved"}))
	var script bytes.Buffer
	result, err := r.Resolve(strings.NewReader(`export MY_ENV=default`), &script)
	require.NoError(t, err)
	require.EqualValues(t, []*EnvResolverResult{
		{Name: "MY_ENV", OriginalValue: "default", Value: "resolved", Prompt: EnvResolverResolved},
	}, result)
}

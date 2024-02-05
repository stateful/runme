package command

import (
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
				{Name: "TEST_NO_VALUE"},
			},
		},
		{
			name: "empty value",
			data: `export TEST_EMPTY_VALUE=`,
			result: []*EnvResolverResult{
				{Name: "TEST_EMPTY_VALUE"},
			},
		},
		{
			name: "string value",
			data: `export TEST_STRING_VALUE=value`,
			result: []*EnvResolverResult{
				{Name: "TEST_STRING_VALUE", OriginalValue: "value"},
			},
		},
		{
			name: "string value with equal sign",
			data: `export TEST_STRING_VALUE_WITH_EQUAL_SIGN=part1=part2`,
			result: []*EnvResolverResult{
				{Name: "TEST_STRING_VALUE_WITH_EQUAL_SIGN", OriginalValue: "part1=part2"},
			},
		},
		{
			name: "string double quoted value empty",
			data: `export TEST_STRING_DBL_QUOTED_VALUE_EMPTY=""`,
			result: []*EnvResolverResult{
				{Name: "TEST_STRING_DBL_QUOTED_VALUE_EMPTY", OriginalValue: ""},
			},
		},
		{
			name: "string double quoted value",
			data: `export TEST_STRING_DBL_QUOTED_VALUE="value"`,
			result: []*EnvResolverResult{
				{Name: "TEST_STRING_DBL_QUOTED_VALUE", OriginalValue: "value"},
			},
		},
		{
			name: "string single quoted value empty",
			data: `export TEST_STRING_SGL_QUOTED_VALUE_EMPTY=''`,
			result: []*EnvResolverResult{
				{Name: "TEST_STRING_SGL_QUOTED_VALUE_EMPTY", OriginalValue: ""},
			},
		},
		{
			name: "string single quoted value",
			data: `export TEST_STRING_SGL_QUOTED_VALUE='value'`,
			result: []*EnvResolverResult{
				{Name: "TEST_STRING_SGL_QUOTED_VALUE", OriginalValue: "value"},
			},
		},
		{
			name: "value expression",
			data: `export TEST_VALUE_EXPR=$(echo -n "value")`,
			result: []*EnvResolverResult{
				{Name: "TEST_VALUE_EXPR"},
			},
		},
		{
			name: "double quoted value expression",
			data: `export TEST_DBL_QUOTE_VALUE_EXPR="$(echo -n 'value')"`,
			result: []*EnvResolverResult{
				{Name: "TEST_DBL_QUOTE_VALUE_EXPR"},
			},
		},
		{
			name: "default value",
			data: `export TEST_DEFAULT_VALUE=${TEST_DEFAULT_VALUE:-value}`,
			result: []*EnvResolverResult{
				{Name: "TEST_DEFAULT_VALUE", OriginalValue: "value"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := NewEnvResolver(tc.source...)
			result, err := r.Resolve(strings.NewReader(tc.data))
			assert.NoError(t, err)
			assert.EqualValues(t, tc.result, result)
		})
	}
}

func TestEnvResolverResolve(t *testing.T) {
	r := NewEnvResolver(EnvResolverSourceFunc([]string{"MY_ENV=resolved"}))
	result, err := r.Resolve(strings.NewReader(`export MY_ENV=default`))
	require.NoError(t, err)
	require.EqualValues(t, []*EnvResolverResult{{Name: "MY_ENV", OriginalValue: "default", Value: "resolved"}}, result)
}

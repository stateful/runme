package command

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type parsingFixture struct {
	name   string
	data   string
	source []EnvResolverSource
	result *EnvResolverResult
}

func runEnvResolverParsingTestCase(tc parsingFixture, mode EnvResolverMode) func(t *testing.T) {
	return func(t *testing.T) {
		r := NewEnvResolver(mode, tc.source...)

		if mode != EnvResolverModeAuto && tc.result != nil {
			tc.result.Prompt = EnvResolverResolved
			tc.result.Value = tc.result.OriginalValue
		}

		var script bytes.Buffer
		result, err := r.Resolve(strings.NewReader(tc.data), &script)
		assert.NoError(t, err)

		var tcres []*EnvResolverResult
		if tc.result != nil {
			tcres = append(tcres, tc.result)
		}

		assert.EqualValues(t, tcres, result)
	}
}

func TestEnvResolverParsing(t *testing.T) {
	fixtures := []parsingFixture{
		{
			name: "no value",
			data: `export TEST_NO_VALUE`,
			result: &EnvResolverResult{
				Name: "TEST_NO_VALUE", Prompt: EnvResolverMessage,
			},
		},
		{
			name: "empty value",
			data: `export TEST_EMPTY_VALUE=`,
			result: &EnvResolverResult{
				Name: "TEST_EMPTY_VALUE", Prompt: EnvResolverMessage,
			},
		},
		{
			name: "string value",
			data: `export TEST_STRING_VALUE=value`,
			result: &EnvResolverResult{
				Name: "TEST_STRING_VALUE", OriginalValue: "value", Prompt: EnvResolverMessage,
			},
		},
		{
			name: "string value with equal sign",
			data: `export TEST_STRING_VALUE_WITH_EQUAL_SIGN=part1=part2`,
			result: &EnvResolverResult{
				Name: "TEST_STRING_VALUE_WITH_EQUAL_SIGN", OriginalValue: "part1=part2", Prompt: EnvResolverMessage,
			},
		},
		{
			name: "string double quoted value empty",
			data: `export TEST_STRING_DBL_QUOTED_VALUE_EMPTY=""`,
			result: &EnvResolverResult{
				Name: "TEST_STRING_DBL_QUOTED_VALUE_EMPTY", OriginalValue: "", Prompt: EnvResolverMessage,
			},
		},
		{
			name: "string double quoted value",
			data: `export TEST_STRING_DBL_QUOTED_VALUE="value"`,
			result: &EnvResolverResult{
				Name: "TEST_STRING_DBL_QUOTED_VALUE", OriginalValue: "value", Prompt: EnvResolverPlaceholder,
			},
		},
		{
			name: "string single quoted value empty",
			data: `export TEST_STRING_SGL_QUOTED_VALUE_EMPTY=''`,
			result: &EnvResolverResult{
				Name: "TEST_STRING_SGL_QUOTED_VALUE_EMPTY", OriginalValue: "", Prompt: EnvResolverPlaceholder,
			},
		},
		{
			name: "string single quoted value",
			data: `export TEST_STRING_SGL_QUOTED_VALUE='value'`,
			result: &EnvResolverResult{
				Name: "TEST_STRING_SGL_QUOTED_VALUE", OriginalValue: "value", Prompt: EnvResolverPlaceholder,
			},
		},
		{
			name:   "parameter expression",
			data:   `export TEST_PARAM_EXPR=${TEST:7:0}`,
			result: nil,
		},
		{
			name:   "arithmetic expression",
			data:   `export TEST_ARITHM_EXPR=$(($z+3))`,
			result: nil,
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

	for _, tc := range fixtures {
		t.Run(
			fmt.Sprintf("Auto_resolve %s", tc.name),
			runEnvResolverParsingTestCase(tc, EnvResolverModeAuto),
		)
	}

	for _, tc := range fixtures {
		t.Run(
			fmt.Sprintf("Skip_resolve %s", tc.name),
			runEnvResolverParsingTestCase(tc, EnvResolverModeSkip),
		)
	}
}

func TestEnvResolverResolve(t *testing.T) {
	r := NewEnvResolver(EnvResolverModeAuto, EnvResolverSourceFunc([]string{"MY_ENV=resolved"}))
	var script bytes.Buffer
	result, err := r.Resolve(strings.NewReader(`export MY_ENV=default`), &script)
	require.NoError(t, err)
	require.EqualValues(t, []*EnvResolverResult{
		{
			Name:          "MY_ENV",
			OriginalValue: "default",
			Value:         "resolved",
			Prompt:        EnvResolverResolved,
		},
	}, result)
}

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
	script string
}

func runEnvResolverFixtureTestCase(tc parsingFixture, mode EnvResolverMode) func(t *testing.T) {
	return func(t *testing.T) {
		r := NewEnvResolver(mode, tc.source...)

		if mode != EnvResolverModeAuto && tc.result != nil {
			tc.result.Prompt = EnvResolverResolved
			tc.result.Value = tc.result.OriginalValue
		}

		var script bytes.Buffer
		result, err := r.Resolve(strings.NewReader(tc.data), &script)
		assert.NoError(t, err)
		assert.EqualValues(t, []byte(tc.script), script.Bytes())

		var tcres []*EnvResolverResult
		if tc.result != nil {
			tcres = append(tcres, tc.result)
		}

		assert.EqualValues(t, tcres, result)
	}
}

func TestEnvResolverAutoAndSkip(t *testing.T) {
	fixtures := []parsingFixture{
		{
			name: "no value",
			data: `export TEST_NO_VALUE`,
			result: &EnvResolverResult{
				Name: "TEST_NO_VALUE", Prompt: EnvResolverMessage,
			},
			script: "#\n# TEST_NO_VALUE set in smart env store\n# \"export TEST_NO_VALUE\"\n\n",
		},
		{
			name: "empty value",
			data: `export TEST_EMPTY_VALUE=`,
			result: &EnvResolverResult{
				Name: "TEST_EMPTY_VALUE", Prompt: EnvResolverMessage,
			},
			script: "#\n# TEST_EMPTY_VALUE set in smart env store\n# \"export TEST_EMPTY_VALUE=\"\n\n",
		},
		{
			name: "string value",
			data: `export TEST_STRING_VALUE=value`,
			result: &EnvResolverResult{
				Name: "TEST_STRING_VALUE", OriginalValue: "value", Prompt: EnvResolverMessage,
			},
			script: "#\n# TEST_STRING_VALUE set in smart env store\n# \"export TEST_STRING_VALUE=value\"\n\n",
		},
		{
			name: "string value with equal sign",
			data: `export TEST_STRING_VALUE_WITH_EQUAL_SIGN=part1=part2`,
			result: &EnvResolverResult{
				Name: "TEST_STRING_VALUE_WITH_EQUAL_SIGN", OriginalValue: "part1=part2", Prompt: EnvResolverMessage,
			},
			script: "#\n# TEST_STRING_VALUE_WITH_EQUAL_SIGN set in smart env store\n# \"export TEST_STRING_VALUE_WITH_EQUAL_SIGN=part1=part2\"\n\n",
		},
		{
			name: "string double quoted value empty",
			data: `export TEST_STRING_DBL_QUOTED_VALUE_EMPTY=""`,
			result: &EnvResolverResult{
				Name: "TEST_STRING_DBL_QUOTED_VALUE_EMPTY", OriginalValue: "", Prompt: EnvResolverMessage,
			},
			script: "#\n# TEST_STRING_DBL_QUOTED_VALUE_EMPTY set in smart env store\n# \"export TEST_STRING_DBL_QUOTED_VALUE_EMPTY=\\\"\\\"\"\n\n",
		},
		{
			name: "string double quoted value",
			data: `export TEST_STRING_DBL_QUOTED_VALUE="value"`,
			result: &EnvResolverResult{
				Name: "TEST_STRING_DBL_QUOTED_VALUE", OriginalValue: "value", Prompt: EnvResolverPlaceholder,
			},
			script: "#\n# TEST_STRING_DBL_QUOTED_VALUE set in smart env store\n# \"export TEST_STRING_DBL_QUOTED_VALUE=\\\"value\\\"\"\n\n",
		},
		{
			name: "string single quoted value empty",
			data: `export TEST_STRING_SGL_QUOTED_VALUE_EMPTY=''`,
			result: &EnvResolverResult{
				Name: "TEST_STRING_SGL_QUOTED_VALUE_EMPTY", OriginalValue: "", Prompt: EnvResolverPlaceholder,
			},
			script: "#\n# TEST_STRING_SGL_QUOTED_VALUE_EMPTY set in smart env store\n# \"export TEST_STRING_SGL_QUOTED_VALUE_EMPTY=''\"\n\n",
		},
		{
			name: "string single quoted value",
			data: `export TEST_STRING_SGL_QUOTED_VALUE='value'`,
			result: &EnvResolverResult{
				Name: "TEST_STRING_SGL_QUOTED_VALUE", OriginalValue: "value", Prompt: EnvResolverPlaceholder,
			},
			script: "#\n# TEST_STRING_SGL_QUOTED_VALUE set in smart env store\n# \"export TEST_STRING_SGL_QUOTED_VALUE='value'\"\n\n",
		},
		{
			name:   "parameter expression",
			data:   `export TEST_PARAM_EXPR=${TEST:7:0}`,
			result: nil,
			script: "export TEST_PARAM_EXPR=${TEST:7:0}\n",
		},
		{
			name:   "arithmetic expression",
			data:   `export TEST_ARITHM_EXPR=$(($z+3))`,
			result: nil,
			script: "export TEST_ARITHM_EXPR=$(($z + 3))\n",
		},
		{
			name:   "value expression",
			data:   `export TEST_VALUE_EXPR=$(echo -n "value")`,
			result: nil,
			script: "export TEST_VALUE_EXPR=$(echo -n \"value\")\n",
		},
		{
			name:   "double quoted value expression",
			data:   `export TEST_DBL_QUOTE_VALUE_EXPR="$(echo -n 'value')"`,
			result: nil,
			script: "export TEST_DBL_QUOTE_VALUE_EXPR=\"$(echo -n 'value')\"\n",
		},
		{
			name:   "default value",
			data:   `export TEST_DEFAULT_VALUE=${TEST_DEFAULT_VALUE:-value}`,
			result: nil,
			script: "export TEST_DEFAULT_VALUE=${TEST_DEFAULT_VALUE:-value}\n",
		},
	}

	for _, tc := range fixtures {
		t.Run(
			fmt.Sprintf("Auto_resolve %s", tc.name),
			runEnvResolverFixtureTestCase(tc, EnvResolverModeAuto),
		)
	}

	for _, tc := range fixtures {
		t.Run(
			fmt.Sprintf("Skip_resolve %s", tc.name),
			runEnvResolverFixtureTestCase(tc, EnvResolverModeSkip),
		)
	}
}

func TestEnvResolverAutoResolve(t *testing.T) {
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

func TestEnvResolverPrompt(t *testing.T) {
	t.Run("Prompt with message", func(t *testing.T) {
		r := NewEnvResolver(EnvResolverModePrompt, EnvResolverSourceFunc([]string{"MY_ENV=resolved"}))
		var script bytes.Buffer
		result, err := r.Resolve(strings.NewReader(`export MY_ENV=default`), &script)
		require.NoError(t, err)
		require.EqualValues(t, []*EnvResolverResult{
			{
				Name:          "MY_ENV",
				OriginalValue: "default",
				Value:         "resolved",
				Prompt:        EnvResolverMessage,
			},
		}, result)
		fmt.Printf("script: %q\n", script.String())
		require.EqualValues(t, []byte("#\n# MY_ENV set in smart env store\n# \"export MY_ENV=default\"\n\n"), script.Bytes())
	})

	t.Run("Prompt with placeholder", func(t *testing.T) {
		r := NewEnvResolver(EnvResolverModePrompt, EnvResolverSourceFunc([]string{"MY_ENV=resolved"}))
		var script bytes.Buffer
		result, err := r.Resolve(strings.NewReader(`export MY_ENV="placeholder value"`), &script)
		require.NoError(t, err)
		require.EqualValues(t, []*EnvResolverResult{
			{
				Name:          "MY_ENV",
				OriginalValue: "placeholder value",
				Value:         "resolved",
				Prompt:        EnvResolverPlaceholder,
			},
		}, result)
		fmt.Printf("script: %q\n", script.String())
		require.EqualValues(t, []byte("#\n# MY_ENV set in smart env store\n# \"export MY_ENV=\\\"placeholder value\\\"\"\n\n"), script.Bytes())
	})
}

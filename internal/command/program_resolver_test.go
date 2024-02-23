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
	source []ProgramResolverSource
	result *ProgramResolverResult
	script string
}

func runProgramResolverFixtureTestCase(tc parsingFixture, mode ProgramResolverMode) func(t *testing.T) {
	return func(t *testing.T) {
		r := NewProgramResolver(mode, tc.source...)

		if mode != ProgramResolverModeAuto && tc.result != nil {
			tc.result.Prompt = ProgramResolverResolved
			tc.result.Value = tc.result.OriginalValue
		}

		var script bytes.Buffer
		result, err := r.Resolve(strings.NewReader(tc.data), &script)
		assert.NoError(t, err)
		assert.EqualValues(t, []byte(tc.script), script.Bytes())

		var tcres []*ProgramResolverResult
		if tc.result != nil {
			tcres = append(tcres, tc.result)
		}

		assert.EqualValues(t, tcres, result)
	}
}

func TestProgramResolverAutoAndSkip(t *testing.T) {
	fixtures := []parsingFixture{
		{
			name: "no value",
			data: `export TEST_NO_VALUE`,
			result: &ProgramResolverResult{
				Name: "TEST_NO_VALUE", Prompt: ProgramResolverMessage,
			},
			script: "#\n# TEST_NO_VALUE set in smart env store\n# \"export TEST_NO_VALUE\"\n\n",
		},
		{
			name: "empty value",
			data: `export TEST_EMPTY_VALUE=`,
			result: &ProgramResolverResult{
				Name: "TEST_EMPTY_VALUE", Prompt: ProgramResolverMessage,
			},
			script: "#\n# TEST_EMPTY_VALUE set in smart env store\n# \"export TEST_EMPTY_VALUE=\"\n\n",
		},
		{
			name: "string value",
			data: `export TEST_STRING_VALUE=value`,
			result: &ProgramResolverResult{
				Name: "TEST_STRING_VALUE", OriginalValue: "value", Prompt: ProgramResolverMessage,
			},
			script: "#\n# TEST_STRING_VALUE set in smart env store\n# \"export TEST_STRING_VALUE=value\"\n\n",
		},
		{
			name: "string value with equal sign",
			data: `export TEST_STRING_VALUE_WITH_EQUAL_SIGN=part1=part2`,
			result: &ProgramResolverResult{
				Name: "TEST_STRING_VALUE_WITH_EQUAL_SIGN", OriginalValue: "part1=part2", Prompt: ProgramResolverMessage,
			},
			script: "#\n# TEST_STRING_VALUE_WITH_EQUAL_SIGN set in smart env store\n# \"export TEST_STRING_VALUE_WITH_EQUAL_SIGN=part1=part2\"\n\n",
		},
		{
			name: "string double quoted value empty",
			data: `export TEST_STRING_DBL_QUOTED_VALUE_EMPTY=""`,
			result: &ProgramResolverResult{
				Name: "TEST_STRING_DBL_QUOTED_VALUE_EMPTY", OriginalValue: "", Prompt: ProgramResolverMessage,
			},
			script: "#\n# TEST_STRING_DBL_QUOTED_VALUE_EMPTY set in smart env store\n# \"export TEST_STRING_DBL_QUOTED_VALUE_EMPTY=\\\"\\\"\"\n\n",
		},
		{
			name: "string double quoted value",
			data: `export TEST_STRING_DBL_QUOTED_VALUE="value"`,
			result: &ProgramResolverResult{
				Name: "TEST_STRING_DBL_QUOTED_VALUE", OriginalValue: "value", Prompt: ProgramResolverPlaceholder,
			},
			script: "#\n# TEST_STRING_DBL_QUOTED_VALUE set in smart env store\n# \"export TEST_STRING_DBL_QUOTED_VALUE=\\\"value\\\"\"\n\n",
		},
		{
			name: "string single quoted value empty",
			data: `export TEST_STRING_SGL_QUOTED_VALUE_EMPTY=''`,
			result: &ProgramResolverResult{
				Name: "TEST_STRING_SGL_QUOTED_VALUE_EMPTY", OriginalValue: "", Prompt: ProgramResolverPlaceholder,
			},
			script: "#\n# TEST_STRING_SGL_QUOTED_VALUE_EMPTY set in smart env store\n# \"export TEST_STRING_SGL_QUOTED_VALUE_EMPTY=''\"\n\n",
		},
		{
			name: "string single quoted value",
			data: `export TEST_STRING_SGL_QUOTED_VALUE='value'`,
			result: &ProgramResolverResult{
				Name: "TEST_STRING_SGL_QUOTED_VALUE", OriginalValue: "value", Prompt: ProgramResolverPlaceholder,
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
			runProgramResolverFixtureTestCase(tc, ProgramResolverModeAuto),
		)
	}

	for _, tc := range fixtures {
		t.Run(
			fmt.Sprintf("Skip_resolve %s", tc.name),
			runProgramResolverFixtureTestCase(tc, ProgramResolverModeSkip),
		)
	}
}

func TestProgramResolverAutoResolve(t *testing.T) {
	r := NewProgramResolver(ProgramResolverModeAuto, ProgramResolverSourceFunc([]string{"MY_ENV=resolved"}))
	var script bytes.Buffer
	result, err := r.Resolve(strings.NewReader(`export MY_ENV=default`), &script)
	require.NoError(t, err)
	require.EqualValues(t, []*ProgramResolverResult{
		{
			Name:          "MY_ENV",
			OriginalValue: "default",
			Value:         "resolved",
			Prompt:        ProgramResolverResolved,
		},
	}, result)
}

func TestProgramResolverPrompt(t *testing.T) {
	t.Run("Prompt with message", func(t *testing.T) {
		r := NewProgramResolver(ProgramResolverModePrompt, ProgramResolverSourceFunc([]string{"MY_ENV=resolved"}))
		var script bytes.Buffer
		result, err := r.Resolve(strings.NewReader(`export MY_ENV=default`), &script)
		require.NoError(t, err)
		require.EqualValues(t, []*ProgramResolverResult{
			{
				Name:          "MY_ENV",
				OriginalValue: "default",
				Value:         "resolved",
				Prompt:        ProgramResolverPlaceholder,
			},
		}, result)
		fmt.Printf("script: %q\n", script.String())
		require.EqualValues(t, []byte("#\n# MY_ENV set in smart env store\n# \"export MY_ENV=default\"\n\n"), script.Bytes())
	})

	t.Run("Prompt with placeholder", func(t *testing.T) {
		r := NewProgramResolver(ProgramResolverModePrompt, ProgramResolverSourceFunc([]string{"MY_ENV=resolved"}))
		var script bytes.Buffer
		result, err := r.Resolve(strings.NewReader(`export MY_ENV="placeholder value"`), &script)
		require.NoError(t, err)
		require.EqualValues(t, []*ProgramResolverResult{
			{
				Name:          "MY_ENV",
				OriginalValue: "placeholder value",
				Value:         "resolved",
				Prompt:        ProgramResolverPlaceholder,
			},
		}, result)
		fmt.Printf("script: %q\n", script.String())
		require.EqualValues(t, []byte("#\n# MY_ENV set in smart env store\n# \"export MY_ENV=\\\"placeholder value\\\"\"\n\n"), script.Bytes())
	})
}

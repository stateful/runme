package command

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProgramResolverResolve(t *testing.T) {
	testCases := []struct {
		name            string
		program         string
		result          *ProgramResolverResult
		modifiedProgram string
	}{
		{
			name:    "no value",
			program: `export TEST_NO_VALUE`,
			result: &ProgramResolverResult{
				ModifiedProgram: true,
				Variables: []ProgramResolverVarResult{
					{
						Status: ProgramResolverStatusUnresolved,
						Name:   "TEST_NO_VALUE",
					},
				},
			},
			modifiedProgram: "#\n# TEST_NO_VALUE set in managed env store\n# \"export TEST_NO_VALUE\"\n\n",
		},
		{
			name:    "empty value",
			program: `export TEST_EMPTY_VALUE=`,
			result: &ProgramResolverResult{
				ModifiedProgram: true,
				Variables: []ProgramResolverVarResult{
					{
						Status: ProgramResolverStatusUnresolved,
						Name:   "TEST_EMPTY_VALUE",
					},
				},
			},
			modifiedProgram: "#\n# TEST_EMPTY_VALUE set in managed env store\n# \"export TEST_EMPTY_VALUE=\"\n\n",
		},
		{
			name:    "string value",
			program: `export TEST_STRING_VALUE=value`,
			result: &ProgramResolverResult{
				ModifiedProgram: true,
				Variables: []ProgramResolverVarResult{
					{
						Status:        ProgramResolverStatusUnresolvedWithMessage,
						Name:          "TEST_STRING_VALUE",
						OriginalValue: "value",
					},
				},
			},
			modifiedProgram: "#\n# TEST_STRING_VALUE set in managed env store\n# \"export TEST_STRING_VALUE=value\"\n\n",
		},
		{
			name:    "string value with equal sign",
			program: `export TEST_STRING_VALUE_WITH_EQUAL_SIGN=part1=part2`,
			result: &ProgramResolverResult{
				ModifiedProgram: true,
				Variables: []ProgramResolverVarResult{
					{
						Status:        ProgramResolverStatusUnresolvedWithMessage,
						Name:          "TEST_STRING_VALUE_WITH_EQUAL_SIGN",
						OriginalValue: "part1=part2",
					},
				},
			},
			modifiedProgram: "#\n# TEST_STRING_VALUE_WITH_EQUAL_SIGN set in managed env store\n# \"export TEST_STRING_VALUE_WITH_EQUAL_SIGN=part1=part2\"\n\n",
		},
		{
			name:    "string double quoted value empty",
			program: `export TEST_STRING_DBL_QUOTED_VALUE_EMPTY=""`,
			result: &ProgramResolverResult{
				ModifiedProgram: true,
				Variables: []ProgramResolverVarResult{
					{
						Status: ProgramResolverStatusUnresolved,
						Name:   "TEST_STRING_DBL_QUOTED_VALUE_EMPTY",
					},
				},
			},
			modifiedProgram: "#\n# TEST_STRING_DBL_QUOTED_VALUE_EMPTY set in managed env store\n# \"export TEST_STRING_DBL_QUOTED_VALUE_EMPTY=\\\"\\\"\"\n\n",
		},
		{
			name:    "string double quoted value",
			program: `export TEST_STRING_DBL_QUOTED_VALUE="value"`,
			result: &ProgramResolverResult{
				ModifiedProgram: true,
				Variables: []ProgramResolverVarResult{
					{
						Status:        ProgramResolverStatusUnresolvedWithPlaceholder,
						Name:          "TEST_STRING_DBL_QUOTED_VALUE",
						OriginalValue: "value",
					},
				},
			},
			modifiedProgram: "#\n# TEST_STRING_DBL_QUOTED_VALUE set in managed env store\n# \"export TEST_STRING_DBL_QUOTED_VALUE=\\\"value\\\"\"\n\n",
		},
		{
			name:    "string single quoted value empty",
			program: `export TEST_STRING_SGL_QUOTED_VALUE_EMPTY=''`,
			result: &ProgramResolverResult{
				ModifiedProgram: true,
				Variables: []ProgramResolverVarResult{
					{
						Status: ProgramResolverStatusUnresolved,
						Name:   "TEST_STRING_SGL_QUOTED_VALUE_EMPTY",
					},
				},
			},
			modifiedProgram: "#\n# TEST_STRING_SGL_QUOTED_VALUE_EMPTY set in managed env store\n# \"export TEST_STRING_SGL_QUOTED_VALUE_EMPTY=''\"\n\n",
		},
		{
			name:    "string single quoted value",
			program: `export TEST_STRING_SGL_QUOTED_VALUE='value'`,
			result: &ProgramResolverResult{
				ModifiedProgram: true,
				Variables: []ProgramResolverVarResult{
					{
						Status:        ProgramResolverStatusUnresolvedWithPlaceholder,
						Name:          "TEST_STRING_SGL_QUOTED_VALUE",
						OriginalValue: "value",
					},
				},
			},
			modifiedProgram: "#\n# TEST_STRING_SGL_QUOTED_VALUE set in managed env store\n# \"export TEST_STRING_SGL_QUOTED_VALUE='value'\"\n\n",
		},
		{
			name:            "parameter expression",
			program:         `export TEST_PARAM_EXPR=${TEST:7:0}`,
			result:          &ProgramResolverResult{},
			modifiedProgram: "export TEST_PARAM_EXPR=${TEST:7:0}\n",
		},
		{
			name:            "arithmetic expression",
			program:         `export TEST_ARITHM_EXPR=$(($z+3))`,
			result:          &ProgramResolverResult{},
			modifiedProgram: "export TEST_ARITHM_EXPR=$(($z + 3))\n",
		},
		{
			name:            "value expression",
			program:         `export TEST_VALUE_EXPR=$(echo -n "value")`,
			result:          &ProgramResolverResult{},
			modifiedProgram: "export TEST_VALUE_EXPR=$(echo -n \"value\")\n",
		},
		{
			name:            "double quoted value expression",
			program:         `export TEST_DBL_QUOTE_VALUE_EXPR="$(echo -n 'value')"`,
			result:          &ProgramResolverResult{},
			modifiedProgram: "export TEST_DBL_QUOTE_VALUE_EXPR=\"$(echo -n 'value')\"\n",
		},
		{
			name:            "default value",
			program:         `export TEST_DEFAULT_VALUE=${TEST_DEFAULT_VALUE:-value}`,
			result:          &ProgramResolverResult{},
			modifiedProgram: "export TEST_DEFAULT_VALUE=${TEST_DEFAULT_VALUE:-value}\n",
		},
	}

	for _, tc := range testCases {
		t.Run("ProgramResolverModeAuto_"+tc.name, func(t *testing.T) {
			r := NewProgramResolver(ProgramResolverModeAuto, []string{})
			buf := bytes.NewBuffer(nil)
			got, err := r.Resolve(strings.NewReader(tc.program), buf)
			require.NoError(t, err)
			assert.EqualValues(t, tc.modifiedProgram, buf.String())
			assert.EqualValues(t, tc.result, got)
		})

		t.Run("ProgramResolverModeSkip_"+tc.name, func(t *testing.T) {
			r := NewProgramResolver(ProgramResolverModeSkipAll, []string{})
			buf := bytes.NewBuffer(nil)
			got, err := r.Resolve(strings.NewReader(tc.program), buf)
			require.NoError(t, err)
			assert.EqualValues(t, tc.modifiedProgram, buf.String())
			// In skip mode, all variables will be resolved.
			if tc.result != nil {
				for i, v := range tc.result.Variables {
					v.Status = ProgramResolverStatusResolved
					v.Value = v.OriginalValue
					tc.result.Variables[i] = v
				}
				assert.EqualValues(t, tc.result, got)
			}
		})
	}
}

func TestProgramResolverResolve_ProgramResolverModeAuto(t *testing.T) {
	r := NewProgramResolver(
		ProgramResolverModeAuto,
		[]string{},
		ProgramResolverSourceFunc([]string{"MY_ENV=resolved"}),
	)
	buf := bytes.NewBuffer(nil)
	result, err := r.Resolve(strings.NewReader(`export MY_ENV=default`), buf)
	require.NoError(t, err)
	require.EqualValues(
		t,
		&ProgramResolverResult{
			ModifiedProgram: true,
			Variables: []ProgramResolverVarResult{
				{
					Status:        ProgramResolverStatusResolved,
					Name:          "MY_ENV",
					OriginalValue: "default",
					Value:         "resolved",
				},
			},
		},
		result,
	)
	require.EqualValues(t, "#\n# MY_ENV set in managed env store\n# \"export MY_ENV=default\"\n\n", buf.String())
}

func TestProgramResolverResolve_ProgramResolverModePrompt(t *testing.T) {
	t.Run("Prompt with message", func(t *testing.T) {
		r := NewProgramResolver(
			ProgramResolverModePromptAll,
			[]string{},
			ProgramResolverSourceFunc([]string{"MY_ENV=resolved"}),
		)
		buf := bytes.NewBuffer(nil)
		result, err := r.Resolve(strings.NewReader(`export MY_ENV=message value`), buf)
		require.NoError(t, err)
		require.EqualValues(
			t,
			&ProgramResolverResult{
				ModifiedProgram: true,
				Variables: []ProgramResolverVarResult{
					{
						Status:        ProgramResolverStatusUnresolvedWithPlaceholder,
						Name:          "MY_ENV",
						OriginalValue: "message value",
						Value:         "resolved",
					},
				},
			},
			result,
		)
		require.EqualValues(t, "#\n# MY_ENV set in managed env store\n# \"export MY_ENV=message value\"\n\n", buf.String())
	})

	t.Run("Prompt with placeholder", func(t *testing.T) {
		r := NewProgramResolver(
			ProgramResolverModePromptAll,
			[]string{},
			ProgramResolverSourceFunc([]string{"MY_ENV=resolved"}),
		)
		buf := bytes.NewBuffer(nil)
		result, err := r.Resolve(strings.NewReader(`export MY_ENV="placeholder value"`), buf)
		require.NoError(t, err)
		require.EqualValues(
			t,
			&ProgramResolverResult{
				ModifiedProgram: true,
				Variables: []ProgramResolverVarResult{
					{
						Status:        ProgramResolverStatusUnresolvedWithPlaceholder,
						Name:          "MY_ENV",
						OriginalValue: "placeholder value",
						Value:         "resolved",
					},
				},
			},
			result,
		)
		require.EqualValues(t, "#\n# MY_ENV set in managed env store\n# \"export MY_ENV=\\\"placeholder value\\\"\"\n\n", buf.String())
	})
}

func TestProgramResolverResolve_SensitiveEnvKeys(t *testing.T) {
	t.Run("Prompt with message", func(t *testing.T) {
		r := NewProgramResolver(
			ProgramResolverModePromptAll,
			[]string{"MY_PASSWORD", "MY_SECRET"},
		)
		buf := bytes.NewBuffer(nil)
		result, err := r.Resolve(strings.NewReader("export MY_PASSWORD=super-secret\nexport MY_SECRET=also-secret\nexport MY_PLAIN=text\n"), buf)
		require.NoError(t, err)
		require.EqualValues(
			t,
			&ProgramResolverResult{
				ModifiedProgram: true,
				Variables: []ProgramResolverVarResult{
					{
						Status:        ProgramResolverStatusUnresolvedWithSecret,
						Name:          "MY_PASSWORD",
						OriginalValue: "super-secret",
						Value:         "",
					},
					{
						Status:        ProgramResolverStatusUnresolvedWithSecret,
						Name:          "MY_SECRET",
						OriginalValue: "also-secret",
						Value:         "",
					},
					{
						Status:        ProgramResolverStatusUnresolvedWithMessage,
						Name:          "MY_PLAIN",
						OriginalValue: "text",
						Value:         "",
					},
				},
			},
			result,
		)
		require.EqualValues(t, "#\n# MY_PASSWORD set in managed env store\n# \"export MY_PASSWORD=super-secret\"\n#\n# MY_SECRET set in managed env store\n# \"export MY_SECRET=also-secret\"\n#\n# MY_PLAIN set in managed env store\n# \"export MY_PLAIN=text\"\n\n", buf.String())
	})
}

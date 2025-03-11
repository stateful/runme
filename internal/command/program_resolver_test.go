package command

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProgramResolverResolve(t *testing.T) {
	testCases := []struct {
		name                 string
		program              string
		result               *ProgramResolverResult
		modifiedProgramFirst string
		modifiedProgramLast  string
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
			modifiedProgramFirst: "# Managed env store retention strategy: first\n\n#\n# TEST_NO_VALUE set in managed env store\n# \"export TEST_NO_VALUE\"\n\n",
			modifiedProgramLast:  "# Managed env store retention strategy: last\n\nexport TEST_NO_VALUE\n",
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
			modifiedProgramFirst: "# Managed env store retention strategy: first\n\n#\n# TEST_EMPTY_VALUE set in managed env store\n# \"export TEST_EMPTY_VALUE=\"\n\n",
			modifiedProgramLast:  "# Managed env store retention strategy: last\n\nexport TEST_EMPTY_VALUE=\n",
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
			modifiedProgramFirst: "# Managed env store retention strategy: first\n\n#\n# TEST_STRING_VALUE set in managed env store\n# \"export TEST_STRING_VALUE=value\"\n\n",
			modifiedProgramLast:  "# Managed env store retention strategy: last\n\nexport TEST_STRING_VALUE=value\n",
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
			modifiedProgramFirst: "# Managed env store retention strategy: first\n\n#\n# TEST_STRING_VALUE_WITH_EQUAL_SIGN set in managed env store\n# \"export TEST_STRING_VALUE_WITH_EQUAL_SIGN=part1=part2\"\n\n",
			modifiedProgramLast:  "# Managed env store retention strategy: last\n\nexport TEST_STRING_VALUE_WITH_EQUAL_SIGN=part1=part2\n",
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
			modifiedProgramFirst: "# Managed env store retention strategy: first\n\n#\n# TEST_STRING_DBL_QUOTED_VALUE_EMPTY set in managed env store\n# \"export TEST_STRING_DBL_QUOTED_VALUE_EMPTY=\\\"\\\"\"\n\n",
			modifiedProgramLast:  "# Managed env store retention strategy: last\n\nexport TEST_STRING_DBL_QUOTED_VALUE_EMPTY=\"\"\n",
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
			modifiedProgramFirst: "# Managed env store retention strategy: first\n\n#\n# TEST_STRING_DBL_QUOTED_VALUE set in managed env store\n# \"export TEST_STRING_DBL_QUOTED_VALUE=\\\"value\\\"\"\n\n",
			modifiedProgramLast:  "# Managed env store retention strategy: last\n\nexport TEST_STRING_DBL_QUOTED_VALUE=\"value\"\n",
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
			modifiedProgramFirst: "# Managed env store retention strategy: first\n\n#\n# TEST_STRING_SGL_QUOTED_VALUE_EMPTY set in managed env store\n# \"export TEST_STRING_SGL_QUOTED_VALUE_EMPTY=''\"\n\n",
			modifiedProgramLast:  "# Managed env store retention strategy: last\n\nexport TEST_STRING_SGL_QUOTED_VALUE_EMPTY=''\n",
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
			modifiedProgramFirst: "# Managed env store retention strategy: first\n\n#\n# TEST_STRING_SGL_QUOTED_VALUE set in managed env store\n# \"export TEST_STRING_SGL_QUOTED_VALUE='value'\"\n\n",
			modifiedProgramLast:  "# Managed env store retention strategy: last\n\nexport TEST_STRING_SGL_QUOTED_VALUE='value'\n",
		},
		{
			name:    "shell-escaped prompt message",
			program: `export TYPE=[Guest type \(hyperv,proxmox,openstack\)]`,
			result: &ProgramResolverResult{
				ModifiedProgram: true,
				Variables: []ProgramResolverVarResult{
					{
						Status:        ProgramResolverStatusUnresolvedWithMessage,
						Name:          "TYPE",
						OriginalValue: "[Guest type (hyperv,proxmox,openstack)]",
					},
				},
			},
			modifiedProgramFirst: "# Managed env store retention strategy: first\n\n#\n# TYPE set in managed env store\n# \"export TYPE=[Guest type \\\\(hyperv,proxmox,openstack\\\\)]\"\n\n",
			modifiedProgramLast:  "# Managed env store retention strategy: last\n\nexport TYPE=[Guest type \\(hyperv,proxmox,openstack\\)]\n",
		},
		{
			name:                 "parameter expression",
			program:              `export TEST_PARAM_EXPR=${TEST:7:0}`,
			result:               &ProgramResolverResult{},
			modifiedProgramFirst: "# Managed env store retention strategy: first\n\nexport TEST_PARAM_EXPR=${TEST:7:0}\n",
			modifiedProgramLast:  "# Managed env store retention strategy: last\n\nexport TEST_PARAM_EXPR=${TEST:7:0}\n",
		},
		{
			name:                 "arithmetic expression",
			program:              `export TEST_ARITHM_EXPR=$(($z+3))`,
			result:               &ProgramResolverResult{},
			modifiedProgramFirst: "# Managed env store retention strategy: first\n\nexport TEST_ARITHM_EXPR=$(($z + 3))\n",
			modifiedProgramLast:  "# Managed env store retention strategy: last\n\nexport TEST_ARITHM_EXPR=$(($z + 3))\n",
		},
		{
			name:                 "value expression",
			program:              `export TEST_VALUE_EXPR=$(echo -n "value")`,
			result:               &ProgramResolverResult{},
			modifiedProgramFirst: "# Managed env store retention strategy: first\n\nexport TEST_VALUE_EXPR=$(echo -n \"value\")\n",
			modifiedProgramLast:  "# Managed env store retention strategy: last\n\nexport TEST_VALUE_EXPR=$(echo -n \"value\")\n",
		},
		{
			name:                 "double quoted value expression",
			program:              `export TEST_DBL_QUOTE_VALUE_EXPR="$(echo -n 'value')"`,
			result:               &ProgramResolverResult{},
			modifiedProgramFirst: "# Managed env store retention strategy: first\n\nexport TEST_DBL_QUOTE_VALUE_EXPR=\"$(echo -n 'value')\"\n",
			modifiedProgramLast:  "# Managed env store retention strategy: last\n\nexport TEST_DBL_QUOTE_VALUE_EXPR=\"$(echo -n 'value')\"\n",
		},
		{
			name:                 "default value",
			program:              `export TEST_DEFAULT_VALUE=${TEST_DEFAULT_VALUE:-value}`,
			result:               &ProgramResolverResult{},
			modifiedProgramFirst: "# Managed env store retention strategy: first\n\nexport TEST_DEFAULT_VALUE=${TEST_DEFAULT_VALUE:-value}\n",
			modifiedProgramLast:  "# Managed env store retention strategy: last\n\nexport TEST_DEFAULT_VALUE=${TEST_DEFAULT_VALUE:-value}\n",
		},
	}

	for _, tc := range testCases {
		strategies := []struct {
			name            string
			strategy        VarRetentionStrategy
			modifiedProgram string
			result          *ProgramResolverResult
		}{
			{
				name:            "First",
				strategy:        VarRetentionStrategyFirst,
				modifiedProgram: tc.modifiedProgramFirst,
				result:          tc.result,
			},
			{
				name:            "Last",
				strategy:        VarRetentionStrategyLast,
				modifiedProgram: tc.modifiedProgramLast,
				result:          nil, // for last strategy variables inside result are always empty
			},
		}

		for _, s := range strategies {
			t.Run(fmt.Sprintf("ProgramResolverModeAuto_%s_%s", s.name, tc.name), func(t *testing.T) {
				r := NewProgramResolver(ProgramResolverModeAuto, []string{})
				buf := bytes.NewBuffer(nil)
				got, err := r.Resolve(strings.NewReader(tc.program), buf, s.strategy)
				require.NoError(t, err)
				assert.EqualValues(t, s.modifiedProgram, buf.String())
				if s.result != nil {
					assert.EqualValues(t, s.result, got)
				}
			})

			t.Run(fmt.Sprintf("ProgramResolverModeSkip_%s_%s", s.name, tc.name), func(t *testing.T) {
				r := NewProgramResolver(ProgramResolverModeSkipAll, []string{})
				buf := bytes.NewBuffer(nil)
				got, err := r.Resolve(strings.NewReader(tc.program), buf, s.strategy)
				require.NoError(t, err)
				if s.strategy == VarRetentionStrategyFirst {
					assert.EqualValues(t, tc.modifiedProgramFirst, buf.String())
				} else {
					assert.EqualValues(t, tc.modifiedProgramLast, buf.String())
				}

				if s.result != nil {
					for i, v := range s.result.Variables {
						v.Status = ProgramResolverStatusResolved
						v.Value = v.OriginalValue
						s.result.Variables[i] = v
					}
					assert.EqualValues(t, s.result, got)
				} else {
					// for last strategy script remains structurally unchanged
					assert.Len(t, got.Variables, 0)
					assert.False(t, got.ModifiedProgram)
				}
			})
		}
	}
}

func TestProgramResolverResolve_ProgramResolverModeAuto(t *testing.T) {
	r := NewProgramResolver(
		ProgramResolverModeAuto,
		[]string{},
		ProgramResolverSourceFunc([]string{"MY_ENV=resolved"}),
	)
	buf := bytes.NewBuffer(nil)
	result, err := r.Resolve(strings.NewReader(`export MY_ENV=default`), buf, VarRetentionStrategyFirst)
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
	require.EqualValues(t, "# Managed env store retention strategy: first\n\n#\n# MY_ENV set in managed env store\n# \"export MY_ENV=default\"\n\n", buf.String())
}

func TestProgramResolverResolve_ProgramResolverModePrompt(t *testing.T) {
	t.Run("Prompt with message", func(t *testing.T) {
		r := NewProgramResolver(
			ProgramResolverModePromptAll,
			[]string{},
			ProgramResolverSourceFunc([]string{"MY_ENV=resolved"}),
		)
		buf := bytes.NewBuffer(nil)
		result, err := r.Resolve(strings.NewReader(`export MY_ENV=message value`), buf, VarRetentionStrategyFirst)
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
		require.EqualValues(t, "# Managed env store retention strategy: first\n\n#\n# MY_ENV set in managed env store\n# \"export MY_ENV=message value\"\n\n", buf.String())
	})

	t.Run("Prompt with placeholder", func(t *testing.T) {
		r := NewProgramResolver(
			ProgramResolverModePromptAll,
			[]string{},
			ProgramResolverSourceFunc([]string{"MY_ENV=resolved"}),
		)
		buf := bytes.NewBuffer(nil)
		result, err := r.Resolve(strings.NewReader(`export MY_ENV="placeholder value"`), buf, VarRetentionStrategyFirst)
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
		require.EqualValues(t, "# Managed env store retention strategy: first\n\n#\n# MY_ENV set in managed env store\n# \"export MY_ENV=\\\"placeholder value\\\"\"\n\n", buf.String())
	})
}

func TestProgramResolverResolve_SensitiveEnvKeys(t *testing.T) {
	t.Run("Prompt with message", func(t *testing.T) {
		r := NewProgramResolver(
			ProgramResolverModePromptAll,
			[]string{"MY_PASSWORD", "MY_SECRET"},
		)
		buf := bytes.NewBuffer(nil)
		result, err := r.Resolve(strings.NewReader("export MY_PASSWORD=super-secret\nexport MY_SECRET=also-secret\nexport MY_PLAIN=text\n"), buf, VarRetentionStrategyFirst)
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
		require.EqualValues(t, "# Managed env store retention strategy: first\n\n#\n# MY_PASSWORD set in managed env store\n# \"export MY_PASSWORD=super-secret\"\n#\n# MY_SECRET set in managed env store\n# \"export MY_SECRET=also-secret\"\n#\n# MY_PLAIN set in managed env store\n# \"export MY_PLAIN=text\"\n\n", buf.String())
	})
}

func TestUnescapeShellLiteral(t *testing.T) {
	assert.Equal(t, `echo "Hello World!"`, unescapeShellLiteral(`echo "Hello World!"`))
	assert.Equal(t, `echo "Hello ${name}!"`, unescapeShellLiteral(`echo "Hello ${name}!"`))
	assert.Equal(t, `[Guest type (hyperv,proxmox,openstack)]`, unescapeShellLiteral(`[Guest type \(hyperv,proxmox,openstack\)]`))
	assert.Equal(t, `[IP of waiting server {foo}]`, unescapeShellLiteral(`[IP of waiting server \{foo\}]`))
	assert.Equal(t, `[Guest\ Type]`, unescapeShellLiteral(`[Guest\ Type]`))
	assert.Equal(t, `[Guest Type]`, unescapeShellLiteral(`\[Guest Type\]`))
}

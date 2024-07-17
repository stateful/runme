package owl

import (
	"strings"
	"testing"

	"github.com/stateful/godotenv"
	"github.com/stretchr/testify/require"
)

func TestMapSpec(t *testing.T) {
	testCases := map[string]struct {
		Values   map[string]string
		Comments map[string]string
		Expected Specs
	}{
		"EmptyComments": {
			Comments: map[string]string{},
			Expected: Specs{},
		},
		"WithSpecs": {
			Values: map[string]string{
				"KEY1": "KEY1",
				"KEY2": "KEY2",
				"KEY3": "KEY3",
				"KEY4": "KEY4",
				"KEY5": "",
			},
			Comments: map[string]string{
				"KEY1": "",
				"KEY2": "Plain",
				"KEY3": "Password",
				"KEY4": "Secret",
				"KEY5": "Plain",
			},
			Expected: Specs{
				"KEY1": {Name: SpecNameOpaque, Valid: false},
				"KEY2": {Name: SpecNamePlain, Valid: true},
				"KEY3": {Name: SpecNamePassword, Valid: true},
				"KEY4": {Name: SpecNameSecret, Valid: true},
				"KEY5": {Name: SpecNamePlain},
			},
		},
		"WithRequiredSpecs": {
			Values: map[string]string{
				"KEY1": "KEY1",
				"KEY2": "KEY2",
				"KEY3": "KEY3",
				"KEY4": "KEY4",
			},
			Comments: map[string]string{
				"KEY1": "!",
				"KEY2": "Plain!",
				"KEY3": "Password!",
				"KEY4": "Secret!",
			},
			Expected: Specs{
				"KEY1": {Name: SpecNameOpaque, Valid: true, Required: true},
				"KEY2": {Name: SpecNamePlain, Valid: true, Required: true},
				"KEY3": {Name: SpecNamePassword, Valid: true, Required: true},
				"KEY4": {Name: SpecNameSecret, Valid: true, Required: true},
			},
		},
		"WithParams": {
			Values: map[string]string{
				"KEY1": "1234567890",
				"KEY2": "1234567890",
			},
			Comments: map[string]string{
				"KEY1": `Password!:{"length":10}`,
				"KEY2": `Password!:{"length":9}`,
			},
			Expected: Specs{
				"KEY1": {Name: SpecNamePassword, Required: true, Valid: true},
				"KEY2": {Name: SpecNamePassword, Required: true},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			specs := ParseRawSpec(tc.Values, tc.Comments)

			if len(specs) != len(tc.Expected) {
				t.Errorf("%s Unexpected number of specs. Expected %d, got %d", name, len(tc.Expected), len(specs))
			}

			for key, expectedSpec := range tc.Expected {
				actualSpec, ok := specs[key]
				if !ok {
					t.Errorf("%s Key %s missing in returned specs", name, key)
				} else if actualSpec != expectedSpec {
					t.Errorf("%s Unexpected spec for key %s. Expected %+v, got %+v", name, key, expectedSpec, actualSpec)
				}
			}
		})
	}
}

func TestUnmarshal(t *testing.T) {
	lines := []string{
		"naked= # Plain",
		`quotedEmpty="" # Plain`,
		`quoted="Foo bar baz" # Plain`,
		`unquoted=unquoted value # Plain`,
		`database=unquoted value # DatabaseUrl`,
	}

	expectedValues := map[string]string{
		"naked":       "",
		"quotedEmpty": "",
		"quoted":      "Foo bar baz",
		"unquoted":    "unquoted value",
		"database":    "unquoted value",
	}
	expectedComments := map[string]string{
		"naked":       "Plain",
		"quotedEmpty": "Plain",
		"quoted":      "Plain",
		"unquoted":    "Plain",
		"database":    "DatabaseUrl",
	}

	bytes := []byte(strings.Join(lines, "\n"))
	values, comments, err := godotenv.UnmarshalBytesWithComments(bytes)
	if err != nil {
		t.Errorf("Unable to parse content %s ", bytes)
	}

	require.Equal(t, expectedValues, values)
	require.Equal(t, expectedComments, comments)
}

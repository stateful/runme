package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigFilter(t *testing.T) {
	testCases := []struct {
		name           string
		typ            string
		condition      string
		env            interface{}
		expectedResult bool
	}{
		{
			name:           "empty block env",
			typ:            FilterTypeBlock,
			condition:      "name != ''",
			env:            FilterBlockEnv{},
			expectedResult: false,
		},
		{
			name:           "empty document env",
			typ:            FilterTypeDocument,
			condition:      "shell != ''",
			env:            FilterDocumentEnv{},
			expectedResult: false,
		},
		{
			name:           "valid name",
			typ:            FilterTypeBlock,
			condition:      "name != ''",
			env:            FilterBlockEnv{Name: "test"},
			expectedResult: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filter := Filter{
				Type:      tc.typ,
				Condition: tc.condition,
			}

			result, err := filter.Evaluate(tc.env)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedResult, result)
		})
	}
}

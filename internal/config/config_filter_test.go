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
		extra          map[string]interface{}
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
		{
			name:      "intersection function in block env",
			typ:       FilterTypeBlock,
			extra:     map[string]interface{}{"tags": []string{"test"}},
			condition: "len(intersection(tags, extra.tags)) > 0",
			env: FilterBlockEnv{
				Tags: []string{"test", "test1", "test2"},
			},
			expectedResult: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filter := Filter{
				Type:      tc.typ,
				Condition: tc.condition,
				Extra:     tc.extra,
			}

			result, err := filter.Evaluate(tc.env)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedResult, result)
		})
	}
}

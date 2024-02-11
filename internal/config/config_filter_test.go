package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigFilter(t *testing.T) {
	filter := Filter{
		Type:      FilterTypeBlock,
		Condition: "name != ''",
	}

	result, err := filter.Evaluate(FilterBlockEnv{})
	require.NoError(t, err)
	assert.False(t, result)

	result, err = filter.Evaluate(FilterBlockEnv{Name: "test"})
	require.NoError(t, err)
	assert.True(t, result)
}

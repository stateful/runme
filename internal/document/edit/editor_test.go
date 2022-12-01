package edit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEditor(t *testing.T) {
	e := new(Editor)
	cells, err := e.Deserialize(testDataNested)
	require.NoError(t, err)
	result, err := e.Serialize(cells)
	require.NoError(t, err)
	assert.Equal(t, string(testDataNested), string(result))
}

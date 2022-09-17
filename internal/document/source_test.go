package document

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSource_Parse(t *testing.T) {
	source := NewSource(testREADME)
	blocks := source.Parse().CodeBlocks()
	assert.Len(t, blocks, 5)

	source, err := NewSourceFromFile(testFS, "README.md")
	require.NoError(t, err)
	blocks = source.Parse().CodeBlocks()
	assert.Len(t, blocks, 5)
}

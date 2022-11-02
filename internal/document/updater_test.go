package document

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpdater_UpdateBlock(t *testing.T) {
	data := []byte(`First paragraph:

## Header 1

` + "```" + `sh
echo 1
` + "```" + `

Second paragraph:

` + "```" + `sh
echo 2
` + "```")
	updater := NewUpdaterWithSource(data)

	err := updater.UpdateBlock(1, "## EDITED")
	assert.NoError(t, err)
	block := updater.parsed.Blocks()[1]
	assert.Equal(t, "## EDITED", block.Content())

	err = updater.UpdateBlock(2, "```sh"+`
echo 3
`+"```")
	assert.NoError(t, err)
	block = updater.parsed.Blocks()[2]
	assert.Equal(t, []string{"echo 3"}, block.(*CodeBlock).Lines())
}

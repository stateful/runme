package md

import (
	"testing"

	"github.com/stateful/runme/internal/document"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRender(t *testing.T) {
	data := []byte(`This is a basic snippet with a shell command:

` + "```" + `sh
$ echo "Hello, runme!"
` + "```" + `

It can have an annotation with a name:

` + "```" + `sh {name=echo first= second=2}
$ echo "Hello, runme!"
` + "```")
	source := document.NewSource(data)
	result, err := Render(source.Parse().Root(), data)
	require.NoError(t, err)
	assert.Equal(t, string(data), result)
}

package edit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEditor(t *testing.T) {
	e := new(Editor)
	notebook, err := e.Deserialize(testDataNested)
	require.NoError(t, err)
	result, err := e.Serialize(notebook)
	require.NoError(t, err)
	assert.Equal(
		t,
		string(testDataNested),
		string(result),
	)
}

func TestEditor_list(t *testing.T) {
	data := []byte(`1. Item 1
2. Item 2
3. Item 3
`)
	e := new(Editor)
	notebook, err := e.Deserialize(data)
	require.NoError(t, err)

	notebook.Cells[0].Value = "1. Item 1\n2. Item 2\n"

	newData, err := e.Serialize(notebook)
	require.NoError(t, err)
	assert.Equal(
		t,
		`1. Item 1
2. Item 2
`,
		string(newData),
	)

	newData, err = e.Serialize(notebook)
	require.NoError(t, err)
	assert.Equal(
		t,
		`1. Item 1
2. Item 2
`,
		string(newData),
	)
}

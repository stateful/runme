package editor

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stateful/runme/internal/document/constants"
)

func TestEditor(t *testing.T) {
	notebook, err := Deserialize(testDataNested)
	require.NoError(t, err)
	result, err := Serialize(notebook)
	require.NoError(t, err)
	assert.Equal(
		t,
		string(testDataNestedFlattened),
		string(result),
	)
}

func TestEditor_List(t *testing.T) {
	data := []byte(`1. Item 1
2. Item 2
3. Item 3
`)
	notebook, err := Deserialize(data)
	require.NoError(t, err)

	notebook.Cells[0].Value = "1. Item 1\n2. Item 2\n"

	newData, err := Serialize(notebook)
	require.NoError(t, err)
	assert.Equal(
		t,
		`1. Item 1
2. Item 2
`,
		string(newData),
	)

	newData, err = Serialize(notebook)
	require.NoError(t, err)
	assert.Equal(
		t,
		`1. Item 1
2. Item 2
`,
		string(newData),
	)
}

func TestEditor_CodeBlock(t *testing.T) {
	t.Run("ProvideGeneratedName", func(t *testing.T) {
		data := []byte("```sh\necho 1\n```\n")
		notebook, err := Deserialize(data)
		require.NoError(t, err)
		cell := notebook.Cells[0]
		assert.Equal(
			t,
			cell.Metadata[PrefixAttributeName(InternalAttributePrefix, "name")],
			"echo-1",
		)
		// "name" is empty because it was not included in the original snippet.
		assert.Empty(
			t,
			cell.Metadata["name"],
		)
		result, err := Serialize(notebook)
		require.NoError(t, err)
		assert.Equal(t, string(data), string(result))
	})

	t.Run("PreserveName", func(t *testing.T) {
		data := []byte("```sh { name = \"name1\" }\necho 1\n```\n")
		notebook, err := Deserialize(data)
		require.NoError(t, err)
		cell := notebook.Cells[0]
		assert.Equal(
			t,
			cell.Metadata[PrefixAttributeName(InternalAttributePrefix, "name")],
			"name1",
		)
		// "name" is not nil because it was included in the original snippet.
		assert.Equal(
			t,
			cell.Metadata["name"],
			"name1",
		)
		result, err := Serialize(notebook)
		require.NoError(t, err)
		assert.Equal(t, string(data), string(result))
	})
}

func TestEditor_FrontMatter(t *testing.T) {
	data := []byte(`+++
prop1 = "val1"
prop2 = "val2"
+++

# Example

A paragraph
`)
	notebook, err := Deserialize(data)
	require.NoError(t, err)
	result, err := Serialize(notebook)
	require.NoError(t, err)
	assert.Equal(
		t,
		string(data),
		string(result),
	)
}

func TestEditor_Newlines(t *testing.T) {
	data := []byte(`## Newline debugging

This will test final line breaks`)

	t.Run("No line breaks", func(t *testing.T) {
		notebook, err := Deserialize(data)
		require.NoError(t, err)

		assert.Equal(
			t,
			notebook.Metadata[PrefixAttributeName(InternalAttributePrefix, constants.FinalLineBreaksKey)],
			"0",
		)

		actual, err := Serialize(notebook)
		require.NoError(t, err)
		assert.Equal(
			t,
			string(actual),
			string(data),
		)
	})

	t.Run("Single line break", func(t *testing.T) {
		withLineBreaks := append(data, byte('\n'))

		notebook, err := Deserialize(withLineBreaks)
		require.NoError(t, err)

		assert.Equal(
			t,
			notebook.Metadata[PrefixAttributeName(InternalAttributePrefix, constants.FinalLineBreaksKey)],
			"1",
		)

		actual, err := Serialize(notebook)
		require.NoError(t, err)
		assert.Equal(
			t,
			string(actual),
			string(withLineBreaks),
		)
	})

	t.Run("7 line breaks", func(t *testing.T) {
		withLineBreaks := append(data, bytes.Repeat([]byte{'\n'}, 7)...)

		notebook, err := Deserialize(withLineBreaks)
		require.NoError(t, err)

		assert.Equal(
			t,
			notebook.Metadata[PrefixAttributeName(InternalAttributePrefix, constants.FinalLineBreaksKey)],
			"7",
		)

		actual, err := Serialize(notebook)
		require.NoError(t, err)
		assert.Equal(
			t,
			string(actual),
			string(withLineBreaks),
		)
	})
}

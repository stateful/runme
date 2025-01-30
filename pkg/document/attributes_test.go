package document

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustReturnAttributes(t *testing.T, format string, items map[string]string) AttributeStore {
	t.Helper()
	attr, err := NewAttributesWithFormat(items, format)
	require.NoError(t, err)
	return attr
}

func TestAttributes(t *testing.T) {
	t.Run("HTMLAttributes", func(t *testing.T) {
		t.Run("Parser", func(t *testing.T) {
			parser := &htmlAttributesParserWriter{}

			testCases := []struct {
				name     string
				input    []byte
				expected AttributeStore
			}{
				{
					name:     "empty",
					input:    []byte("{}"),
					expected: mustReturnAttributes(t, "html", nil),
				},
				{
					name:     "valid",
					input:    []byte("{ hello=world key=value val=20 }"),
					expected: mustReturnAttributes(t, "html", map[string]string{"hello": "world", "key": "value", "val": "20"}),
				},
				{
					name:     "empty-attribute",
					input:    []byte("{ hello=world val=20 empty }"),
					expected: mustReturnAttributes(t, "html", map[string]string{"hello": "world", "val": "20"}),
				},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					attr, err := parser.Parse(tc.input)
					require.NoError(t, err)
					assert.Equal(t, tc.expected, attr)
				})
			}
		})

		t.Run("Writer", func(t *testing.T) {
			parser := &htmlAttributesParserWriter{}

			testCases := []struct {
				name     string
				input    AttributeStore
				expected string
			}{
				{
					name:     "empty",
					input:    mustReturnAttributes(t, "html", nil),
					expected: "{}",
				},
				{
					name:     "valid",
					input:    mustReturnAttributes(t, "html", map[string]string{"hello": "world", "key": "value", "val": "20", "name": "script"}),
					expected: "{ name=script hello=world key=value val=20 }",
				},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					buf := bytes.NewBuffer([]byte{})
					err := parser.Write(buf, tc.input)
					require.NoError(t, err)
					assert.Equal(t, tc.expected, buf.String())
				})
			}
		})
	})

	t.Run("JSON", func(t *testing.T) {
		t.Run("Parser", func(t *testing.T) {
			parser := &jsonParserWriter{}

			testCases := []struct {
				name  string
				input []byte

				expected AttributeStore
			}{
				{
					name:     "empty",
					input:    []byte("{}"),
					expected: mustReturnAttributes(t, "json", nil),
				},
				{
					name:     "valid",
					input:    []byte("{\"hello\":\"world\",\"key\":\"value\",\"val\":\"20\"}"),
					expected: mustReturnAttributes(t, "json", map[string]string{"hello": "world", "key": "value", "val": "20"}),
				},
				{
					name:     "nested",
					input:    []byte("{\"nested\":{\"hello\":\"world\"}}"),
					expected: mustReturnAttributes(t, "json", map[string]string{"nested": "{\"hello\":\"world\"}"}),
				},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					attr, err := parser.Parse(tc.input)
					require.NoError(t, err)
					assert.Equal(t, tc.expected, attr)
				})
			}
		})

		t.Run("Writer", func(t *testing.T) {
			writer := &jsonParserWriter{}

			testCases := []struct {
				name     string
				input    AttributeStore
				expected string
			}{
				{
					name:     "empty",
					input:    mustReturnAttributes(t, "json", nil),
					expected: "{}",
				},
				{
					name:     "valid",
					input:    mustReturnAttributes(t, "json", map[string]string{"hello": "world", "key": "value", "val": "20", "name": "script"}),
					expected: "{\"hello\":\"world\",\"key\":\"value\",\"name\":\"script\",\"val\":\"20\"}",
				},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					buf := bytes.NewBuffer(nil)
					err := writer.Write(buf, tc.input)
					require.NoError(t, err)
					assert.Equal(t, tc.expected, buf.String())
				})
			}
		})
	})
}

func TestWriteAttributes_parseAttributes(t *testing.T) {
	t.Run("JSONToJSON", func(t *testing.T) {
		src := []byte("{\"float\":13.3,\"key\":\"value\",\"val\":20}")

		attr, err := parseAttributes(src)
		require.NoError(t, err)

		assert.Equal(t, mustReturnAttributes(t, "json", map[string]string{
			"key":   "value",
			"val":   "20",
			"float": "13.3",
		}), attr)

		buf := bytes.NewBuffer(nil)
		err = WriteAttributes(buf, attr)
		require.NoError(t, err)
		assert.Equal(t, "{\"float\":\"13.3\",\"key\":\"value\",\"val\":\"20\"}", buf.String())
	})

	t.Run("HTMLAttributesToJSON", func(t *testing.T) {
		src := []byte("{ float=13.3 key=value val=20 }")

		attr, err := parseAttributes(src)
		require.NoError(t, err)

		expected := mustReturnAttributes(t, "json", map[string]string{
			"key":   "value",
			"val":   "20",
			"float": "13.3",
		})
		assert.Equal(t, expected.Items(), attr.Items())

		buf := bytes.NewBuffer(nil)
		err = WriteAttributes(buf, attr)
		require.NoError(t, err)
		assert.Equal(t, "{ float=13.3 key=value val=20 }", buf.String())
	})
}

func TestAttributes_Retain(t *testing.T) {
	t.Run("RetainFormat", func(t *testing.T) {
		src := []byte("{ interactive=false name=date }")

		attr, err := parseAttributes(src)
		require.NoError(t, err)

		assert.Equal(t, map[string]string{
			"interactive": "false",
			"name":        "date",
		}, attr.Items())

		buf := bytes.NewBuffer(nil)
		err = WriteAttributes(buf, attr)
		require.NoError(t, err)
		assert.Equal(t, "{ name=date interactive=false }", buf.String())
	})
}

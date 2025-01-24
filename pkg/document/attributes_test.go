package document

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttributes(t *testing.T) {
	t.Run("HTMLAttributes", func(t *testing.T) {
		t.Run("Parser", func(t *testing.T) {
			parser := &htmlAttributesParserWriter{}

			testCases := []struct {
				name     string
				input    []byte
				expected Attributes
			}{
				{
					name:     "empty",
					input:    []byte("{}"),
					expected: Attributes{},
				},
				{
					name:     "valid",
					input:    []byte("{ hello=world key=value val=20 }"),
					expected: Attributes{"hello": "world", "key": "value", "val": "20"},
				},
				{
					name:     "empty-attribute",
					input:    []byte("{ hello=world val=20 empty }"),
					expected: Attributes{"hello": "world", "val": "20"},
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
				input    Attributes
				expected string
			}{
				{
					name:     "empty",
					input:    nil,
					expected: "{}",
				},
				{
					name:     "valid",
					input:    Attributes{"hello": "world", "key": "value", "val": "20", "name": "script"},
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

				expected Attributes
			}{
				{
					name:     "empty",
					input:    []byte("{}"),
					expected: Attributes{},
				},
				{
					name:     "valid",
					input:    []byte("{\"hello\":\"world\",\"key\":\"value\",\"val\":\"20\"}"),
					expected: Attributes{"hello": "world", "key": "value", "val": "20"},
				},
				{
					name:     "nested",
					input:    []byte("{\"nested\":{\"hello\":\"world\"}}"),
					expected: Attributes{"nested": "{\"hello\":\"world\"}"},
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
				input    Attributes
				expected string
			}{
				{
					name:     "empty",
					input:    Attributes{},
					expected: "{}",
				},
				{
					name:     "valid",
					input:    Attributes{"hello": "world", "key": "value", "val": "20", "name": "script"},
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

		assert.Equal(t, Attributes{
			"key":   "value",
			"val":   "20",
			"float": "13.3",
		}, attr)

		buf := bytes.NewBuffer(nil)
		err = WriteAttributes(buf, attr)
		require.NoError(t, err)
		assert.Equal(t, "{\"float\":\"13.3\",\"key\":\"value\",\"val\":\"20\"}", buf.String())
	})

	t.Run("HTMLAttributesToJSON", func(t *testing.T) {
		src := []byte("{ float=13.3 key=value val=20 }")

		attr, err := parseAttributes(src)
		require.NoError(t, err)

		assert.Equal(t, Attributes{
			"key":   "value",
			"val":   "20",
			"float": "13.3",
		}, attr)

		buf := bytes.NewBuffer(nil)
		err = WriteAttributes(buf, attr)
		require.NoError(t, err)
		assert.Equal(t, "{\"float\":\"13.3\",\"key\":\"value\",\"val\":\"20\"}", buf.String())
	})
}

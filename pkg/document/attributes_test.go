package document

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttributes(t *testing.T) {
	t.Run("HTML", func(t *testing.T) {
		t.Run("Parser", func(t *testing.T) {
			parser := &htmlAttrParserWriter{}

			testCases := []struct {
				name     string
				input    []byte
				expected *Attributes
			}{
				{
					name:     "Empty",
					input:    []byte("{}"),
					expected: NewAttributesWithFormat(nil, "html"),
				},
				{
					name:     "Valid",
					input:    []byte("{ hello=world key=value val=20 }"),
					expected: NewAttributesWithFormat(map[string]string{"hello": "world", "key": "value", "val": "20"}, "html"),
				},
				{
					name:     "AttributeWithoutValue",
					input:    []byte("{ hello=world val=20 empty }"),
					expected: NewAttributesWithFormat(map[string]string{"hello": "world", "val": "20", "empty": ""}, "html"),
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
			parser := &htmlAttrParserWriter{}

			testCases := []struct {
				name     string
				input    *Attributes
				expected string
			}{
				{
					name:     "empty",
					input:    NewAttributesWithFormat(nil, "html"),
					expected: "{}",
				},
				{
					name: "valid",
					input: NewAttributesWithFormat(
						map[string]string{"hello": "world", "key": "value", "val": "20", "name": "script"},
						"html",
					),
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
			parser := &jsonAttrParserWriter{}

			testCases := []struct {
				name     string
				input    []byte
				expected *Attributes
			}{
				{
					name:     "empty",
					input:    []byte("{}"),
					expected: NewAttributesWithFormat(nil, "json"),
				},
				{
					name:     "valid",
					input:    []byte("{\"hello\":\"world\",\"key\":\"value\",\"val\":\"20\"}"),
					expected: NewAttributesWithFormat(map[string]string{"hello": "world", "key": "value", "val": "20"}, "json"),
				},
				{
					name:     "nested",
					input:    []byte("{\"nested\":{\"hello\":\"world\"}}"),
					expected: NewAttributesWithFormat(map[string]string{"nested": "{\"hello\":\"world\"}"}, "json"),
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
			writer := &jsonAttrParserWriter{}

			testCases := []struct {
				name     string
				input    *Attributes
				expected string
			}{
				{
					name:     "empty json",
					input:    NewAttributesWithFormat(nil, "json"),
					expected: "{}",
				},
				{
					name: "valid",
					input: NewAttributesWithFormat(
						map[string]string{"hello": "world", "key": "value", "val": "20", "name": "script"},
						"json",
					),
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

func TestAttributes_Conversion(t *testing.T) {
	t.Run("RetainJSON", func(t *testing.T) {
		src := []byte(`{"float":"13.3","key":"value","val":"20"}`)

		attr, err := parseAttributes(src)
		require.NoError(t, err)

		buf := bytes.NewBuffer(nil)
		err = WriteAttributes(buf, attr)
		require.NoError(t, err)
		assert.Equal(t, string(src), buf.String())
	})

	t.Run("RetainHTMLAttributes", func(t *testing.T) {
		src := []byte("{ float=13.3 key=value val=20 }")

		attr, err := parseAttributes(src)
		require.NoError(t, err)

		buf := bytes.NewBuffer(nil)
		err = WriteAttributes(buf, attr)
		require.NoError(t, err)
		assert.Equal(t, string(src), buf.String())
	})
}

// In the case of invalid JSON, we fallback to HTML attributes.
// It might not make much sense, but that's the current behavior.
func TestAttributes_Fallback(t *testing.T) {
	src := []byte(`{ name: "test" }`) // invalid JSON

	attr, err := parseAttributes(src)
	require.NoError(t, err)
	assert.Equal(t, "html", attr.Format)

	buf := bytes.NewBuffer(nil)
	err = WriteAttributes(buf, attr)
	require.NoError(t, err)
	assert.NotEmpty(t, buf.String())
}

package document

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_attributes(t *testing.T) {
	t.Run("BabikML", func(t *testing.T) {
		parser := &babikMLParser{}

		// parser
		{
			src := []byte("{ hello=world key=value val=20 }")

			attr, err := parser.Parse(src)
			require.NoError(t, err)

			assert.Equal(t, Attributes{
				"key":   "value",
				"hello": "world",
				"val":   "20",
			}, attr)

			serialized := bytes.NewBuffer(nil)
			parser.Write(attr, serialized)
			assert.Equal(t, string(src), serialized.String())
		}

		{
			attr, err := parser.Parse([]byte("{ hello=world val=20 empty }"))
			require.NoError(t, err)

			assert.Equal(t, Attributes{
				"hello": "world",
				"val":   "20",
			}, attr)
		}

		// writer
		{
			attr := Attributes{
				"key":   "value",
				"val":   "20",
				"float": "13.3",
				"name":  "script",
			}

			w := bytes.NewBuffer([]byte{})
			err := parser.Write(attr, w)
			require.NoError(t, err)

			result := w.String()
			assert.Equal(t, "{ name=script float=13.3 key=value val=20 }", result)

			parsed, err := parser.Parse([]byte(result))
			require.NoError(t, err)
			assert.Equal(t, attr, parsed)
		}
	})

	t.Run("JSON", func(t *testing.T) {
		parser := &jsonParser{}

		// parser
		{
			src := []byte("{\"float\":13.3,\"key\":\"value\",\"val\":20}")

			attr, err := parser.Parse(src)
			require.NoError(t, err)

			assert.Equal(t, Attributes{
				"key":   "value",
				"val":   "20",
				"float": "13.3",
			}, attr)

			serialized := bytes.NewBuffer(nil)
			parser.Write(attr, serialized)
			assert.Equal(t, "{\"float\":\"13.3\",\"key\":\"value\",\"val\":\"20\"}", serialized.String())
		}

		{
			attr, err := parser.Parse([]byte("{\"nested\":{\"hello\":\"world\"}}"))
			require.NoError(t, err)

			assert.Equal(t, Attributes{
				"nested": "{\"hello\":\"world\"}",
			}, attr)
		}

		// writer
		{
			attr := Attributes{
				"key":   "value",
				"val":   "20",
				"float": "13.3",
				"name":  "script",
				// (supports spaces)
				"zebras": "are cool",
			}

			w := bytes.NewBuffer([]byte{})
			err := parser.Write(attr, w)
			require.NoError(t, err)

			result := w.String()
			assert.Equal(t, "{\"float\":\"13.3\",\"key\":\"value\",\"name\":\"script\",\"val\":\"20\",\"zebras\":\"are cool\"}", result)

			parsed, err := parser.Parse([]byte(result))
			require.NoError(t, err)
			assert.Equal(t, attr, parsed)
		}
	})

	t.Run("failoverAttributesParser", func(t *testing.T) {
		parser := DefaultAttributeParser

		// parser handles json
		{
			src := []byte("{\"float\":13.3,\"key\":\"value\",\"val\":20}")

			attr, err := parser.Parse(src)
			require.NoError(t, err)

			assert.Equal(t, Attributes{
				"key":   "value",
				"val":   "20",
				"float": "13.3",
			}, attr)

			serialized := bytes.NewBuffer(nil)
			parser.Write(attr, serialized)
			assert.Equal(t, "{\"float\":\"13.3\",\"key\":\"value\",\"val\":\"20\"}", serialized.String())
		}

		// parser handles babikml
		{
			src := []byte("{ float=13.3 key=value val=20 }")

			attr, err := parser.Parse(src)
			require.NoError(t, err)

			assert.Equal(t, Attributes{
				"key":   "value",
				"val":   "20",
				"float": "13.3",
			}, attr)

			serialized := bytes.NewBuffer(nil)
			parser.Write(attr, serialized)
			assert.Equal(t, "{\"float\":\"13.3\",\"key\":\"value\",\"val\":\"20\"}", serialized.String())
		}
	})
}

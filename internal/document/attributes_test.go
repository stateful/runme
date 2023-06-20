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
			attr, err := parser.Parse([]byte("{ key=value hello=world val=20 }"))
			require.NoError(t, err)

			assert.Equal(t, Attributes{
				"key":   "value",
				"hello": "world",
				"val":   "20",
			}, attr)
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

			assert.Equal(t, "{ name=script float=13.3 key=value val=20 }", w.String())
		}
	})

	t.Run("TOML", func(t *testing.T) {
		parser := &tomlParser{}

		// parser
		{
			attr, err := parser.Parse([]byte("{ key=\"value\", val=20, float=13.3 }"))
			require.NoError(t, err)

			assert.Equal(t, Attributes{
				"key":   "value",
				"val":   "20",
				"float": "13.3",
			}, attr)
		}

		{
			attr, err := parser.Parse([]byte("{ nested={ hello=\"world\" } }"))
			require.NoError(t, err)

			assert.Equal(t, Attributes{}, attr)
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

			assert.Equal(t, "{ name = \"script\", float = \"13.3\", key = \"value\", val = \"20\", zebras = \"are cool\" }", w.String())
		}
	})
}

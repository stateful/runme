package owl

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_RawParsing(t *testing.T) {
	t.Parallel()

	t.Run("mandatory with spec", func(t *testing.T) {
		raw := []byte(`OPENAI_API_KEY=OpenAI API key matching the org # Password!`)
		k, v, s, m := parseRawSpec(raw)
		assert.EqualValues(t, k, "OPENAI_API_KEY")
		assert.EqualValues(t, v, "OpenAI API key matching the org")
		assert.EqualValues(t, s, "Password")
		assert.EqualValues(t, m, true)
	})

	t.Run("mandatory without spec", func(t *testing.T) {
		raw := []byte(`FRONTEND_URL=http://localhost:4001 #!`)
		k, v, s, m := parseRawSpec(raw)
		assert.EqualValues(t, k, "FRONTEND_URL")
		assert.EqualValues(t, v, "http://localhost:4001")
		assert.EqualValues(t, s, "")
		assert.EqualValues(t, m, true)
	})

	t.Run("optional with sepc", func(t *testing.T) {
		raw := []byte(`CRYPTOGRAPHY_KEY= # Secret`)
		k, v, s, m := parseRawSpec(raw)
		assert.EqualValues(t, k, "CRYPTOGRAPHY_KEY")
		assert.EqualValues(t, v, "")
		assert.EqualValues(t, s, "Secret")
		assert.EqualValues(t, m, false)
	})

	t.Run("optional without spec", func(t *testing.T) {
		raw := []byte(`VECTOR_DB_COLLECTION=internal11`)
		k, v, s, m := parseRawSpec(raw)
		assert.EqualValues(t, k, "VECTOR_DB_COLLECTION")
		assert.EqualValues(t, v, "internal11")
		assert.EqualValues(t, s, "")
		assert.EqualValues(t, m, false)
	})

	// t.Run("dot env", func(t *testing.T) {
	// 	raw, err := os.ReadFile("../../pkg/project/test_project/.env")
	// 	require.NoError(t, err)

	// 	err = parseEnvSpecs(raw)
	// 	require.NoError(t, err)
	// })
}

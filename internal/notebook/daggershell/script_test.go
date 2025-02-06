package daggershell

import (
	"bytes"
	"strings"
	"testing"

	"github.com/go-playground/assert/v2"
	"github.com/stretchr/testify/require"
)

func TestDaggerShell_FuncDecl(t *testing.T) {
	script := NewScript()

	err := script.DeclareFunc("DAGGER_FUNCTION", `echo "Dagger Function Placeholder"`)
	require.NoError(t, err)

	var rendered bytes.Buffer
	err = script.Render(&rendered)
	require.NoError(t, err)

	const expected = `DAGGER_FUNCTION()
{
  echo "Dagger Function Placeholder"
}
`
	assert.Equal(t,
		expected,
		rendered.String(),
	)
}

func TestDaggerShell_Script(t *testing.T) {
	// can't use map because order is not guaranteed
	fakeCells := []struct {
		Name string
		Body string
	}{
		{"DAGGER_01JJDCG2SQSGV0DP55X86EJFSZ", `echo "Use known ID"; date;`},
		{"PRESETUP", `echo "This is PRESETUP" | xxd`},
		{"EXTENSION", `echo "This is EXTENSION" | less`},
		{"KERNEL_BINARY", `echo "This is KERNEL_BINARY"`},
	}

	expected := `DAGGER_01JJDCG2SQSGV0DP55X86EJFSZ()
{
  echo "Use known ID"
  date
}
PRESETUP()
{
  echo "This is PRESETUP" | xxd
}
EXTENSION()
{
  echo "This is EXTENSION" | less
}
KERNEL_BINARY()
{
  echo "This is KERNEL_BINARY"
}
`

	t.Run("Render", func(t *testing.T) {
		script := NewScript()
		for _, entry := range fakeCells {
			script.DeclareFunc(entry.Name, entry.Body)
		}

		var rendered bytes.Buffer
		err := script.Render(&rendered)
		require.NoError(t, err)

		assert.Equal(t, expected, rendered.String())
	})

	t.Run("RenderWithCall", func(t *testing.T) {
		script := NewScript()
		for _, entry := range fakeCells {
			err := script.DeclareFunc(entry.Name, entry.Body)
			require.NoError(t, err)
		}

		for _, entry := range fakeCells {
			var renderedWithCall bytes.Buffer
			err := script.RenderWithCall(&renderedWithCall, entry.Name)
			require.NoError(t, err)

			// add function call padded by new lines
			expectedBytesWithCall := strings.Join([]string{expected[:len(expected)-1], entry.Name, ""}, "\n")
			assert.Equal(t, expectedBytesWithCall, renderedWithCall.String())
		}
	})

	t.Run("RenderWithCall_Invalid", func(t *testing.T) {
		script := NewScript()
		for _, entry := range fakeCells {
			err := script.DeclareFunc(entry.Name, entry.Body)
			require.NoError(t, err)
		}

		var renderedWithCall bytes.Buffer
		err := script.RenderWithCall(&renderedWithCall, "INVALID")
		require.Error(t, err)
	})
}

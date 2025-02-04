package notebook

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/go-playground/assert/v2"
	"github.com/stretchr/testify/require"
)

func TestDaggerShell_FuncDecl(t *testing.T) {
	script := NewScript()

	err := script.declareFunc("DAGGER_FUNCTION", `echo "Dagger Function Placeholder"`)
	require.NoError(t, err)

	var rendered bytes.Buffer
	err = script.Render(&rendered)
	require.NoError(t, err)

	assert.Equal(t,
		"DAGGER_FUNCTION() { echo \"Dagger Function Placeholder\"; }\n",
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

	expected := strings.Join([]string{
		"DAGGER_01JJDCG2SQSGV0DP55X86EJFSZ() { echo \"Use known ID\"; date; };",
		"PRESETUP() { echo \"This is PRESETUP\" | xxd; };",
		"EXTENSION() { echo \"This is EXTENSION\" | less; };",
		"KERNEL_BINARY() { echo \"This is KERNEL_BINARY\"; }\n",
	}, " ")

	t.Run("Render", func(t *testing.T) {
		script := NewScript()
		for _, entry := range fakeCells {
			script.declareFunc(entry.Name, entry.Body)
		}

		var rendered bytes.Buffer
		err := script.Render(&rendered)
		require.NoError(t, err)

		assert.Equal(t, expected, rendered.String())
	})

	t.Run("RenderWithCall", func(t *testing.T) {
		script := NewScript()
		for _, entry := range fakeCells {
			script.declareFunc(entry.Name, entry.Body)
		}

		for _, entry := range fakeCells {
			var renderedWithCall bytes.Buffer
			err := script.RenderWithCall(&renderedWithCall, entry.Name)
			require.NoError(t, err)

			expectedWithCall := fmt.Sprintf("%s; %s\n", strings.Trim(expected, "\n"), entry.Name)
			assert.Equal(t, expectedWithCall, renderedWithCall.String())
			fmt.Println(renderedWithCall.String())
		}
	})
}

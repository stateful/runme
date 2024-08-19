package command

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHostRuntime(t *testing.T) {
	t.Run("WithSystem", func(t *testing.T) {
		r := hostRuntime{
			fixedEnviron: []string{"A=1", "B=2"},
			useSystem:    true,
		}
		require.EqualValues(t, append([]string{"A=1", "B=2"}, os.Environ()...), r.Environ())
	})

	t.Run("WithoutSystem", func(t *testing.T) {
		r := hostRuntime{
			fixedEnviron: []string{"A=1", "B=2"},
			useSystem:    false,
		}
		require.EqualValues(t, []string{"A=1", "B=2"}, r.Environ())
	})
}

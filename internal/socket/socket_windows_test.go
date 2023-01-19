package socket

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_fixPath(t *testing.T) {
	require.Equal(t, `\\.\pipe\tmp-app.stateful`, fixPath("/tmp/app.stateful"))
}

//go:build !windows

package kernel

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectPrompt(t *testing.T) {
	bashBin, err := exec.LookPath("bash")
	require.NoError(t, err)
	prompt, err := DetectPrompt(bashBin)
	require.NoError(t, err)
	assert.Regexp(t, regexp.MustCompile("^bash.*$"), string(prompt))
}

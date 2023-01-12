//go:build !windows

package kernel

import (
	"os"
	"os/exec"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectPrompt(t *testing.T) {
	var promptRe *regexp.Regexp

	if os.Getenv("CI") == "true" {
		promptRe = regexp.MustCompile("^runner@.*:.*$")
	} else {
		promptRe = regexp.MustCompile("^bash-.*$")
	}

	bashBin, err := exec.LookPath("bash")
	require.NoError(t, err)
	prompt, err := DetectPrompt(bashBin)
	require.NoError(t, err)
	assert.Regexp(t, promptRe, string(prompt))
}

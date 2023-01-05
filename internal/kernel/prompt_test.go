//go:build !windows

package kernel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectPrompt(t *testing.T) {
	prompt, err := DetectPrompt("/usr/local/bin/bash")
	require.NoError(t, err)
	assert.Equal(t, "bash-5.2$", string(prompt))
}

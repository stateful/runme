package extension

import (
	"bytes"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

var allExtensionNames = []string{DefaultExtensionName}

func Test_commandExists(t *testing.T) {
	if runtime.GOOS == "windows" {
		result := commandExists("cmd.exe")
		require.True(t, result)
	} else {
		result := commandExists("sh")
		require.True(t, result)
	}
}

func Test_isExtensionInstalled(t *testing.T) {
	// No extensions installed.
	version, result, err := isInstalled(nil, allExtensionNames)
	require.NoError(t, err)
	require.False(t, result)
	require.Empty(t, version)

	// We currently don't have any legacy names
	// Legacy extension installed.
	var extensions []ext
	for _, name := range allExtensionNames {
		if name != DefaultExtensionName {
			extensions = append(extensions, ext{Name: name})
		}
	}
	version, result, err = isInstalled(extensions, allExtensionNames)
	require.NoError(t, err)
	// Without legacy extensions this needs to be falsey
	require.False(t, result)
	require.Empty(t, version)

	// Default extension installed.
	extensions = []ext{{Name: DefaultExtensionName}}
	version, result, err = isInstalled(extensions, allExtensionNames)
	require.NoError(t, err)
	require.True(t, result)
	require.NotEmpty(t, version)
}

func Test_parseExtensions(t *testing.T) {
	output := bytes.NewBufferString(`Extensions installed on Codespaces:
GitHub.codespaces@1.3.6
GitHub.vscode-pull-request-github@0.32.0
golang.go@0.29.0`)
	extensions, err := parseExtensions(output)
	require.NoError(t, err)
	require.Len(t, extensions, 3)
	require.Equal(t, ext{Name: "GitHub.codespaces", Version: "1.3.6"}, extensions[0])
}

//go:test !darwin

package cmd

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_getExtensionURL(t *testing.T) {
	extensionURL, err := getExtensionURL("v1.7.0")
	assert.NoError(t, err)

	regex := regexp.MustCompile(`https:\/\/github.com\/stateful\/vscode-runme\/releases\/download\/(.*)\/(.*)`)
	require.Equal(
		t,
		regex.Match([]byte(extensionURL)),
		true,
	)
}

func Test_getLatestExtensionVersion(t *testing.T) {
	latestStable, err := getLatestExtensionVersion(false)
	require.NoError(t, err)
	assert.NotEmpty(t, latestStable)

	latestPreview, err := getLatestExtensionVersion(false)
	require.NoError(t, err)
	assert.NotEmpty(t, latestPreview)
}

func Test_downloadVscodeExtension(t *testing.T) {
	tmpDir := os.TempDir()

	rootFolder := filepath.Join(tmpDir, "testing_runme_download_vscode")
	_ = os.MkdirAll(rootFolder, 0o700)
	defer os.RemoveAll(rootFolder)

	fileName, err := downloadVscodeExtension(rootFolder, false)
	require.NoError(t, err)

	fi, err := os.Stat(fileName)
	require.NoError(t, err)

	assert.Equal(t, fi.IsDir(), false)
}

func Test_vscodeCodeServerVersionRegexp(t *testing.T) {
	assert.Equal(
		t,
		vscodeVersionRegexp.MatchString("VS Code Server 20220909.27"),
		true,
	)

	assert.Equal(
		t,
		vscodeVersionRegexp.MatchString("4.14.1 5c199629305a0b935b4388b7db549f77eae82b5a with Code 1.79.2"),
		false,
	)
}

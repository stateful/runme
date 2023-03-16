package runner

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrepareScript(t *testing.T) {
	script := prepareScriptFromCommands([]string{
		`# macOS`,
		`brew bundle --no-lock`,
		`brew upgrade`,
	}, "bash")
	assert.Equal(t, "set -e -o pipefail\n# macOS\nbrew bundle --no-lock\nbrew upgrade\n\n", script)

	script = prepareScriptFromCommands([]string{
		"deno install \\",
		"--allow-read --allow-write \\",
		"--allow-env --allow-net --allow-run \\",
		"--no-check \\",
		"-r -f https://deno.land/x/deploy/deployctl.ts",
	}, "bash")
	assert.Equal(t, "set -e -o pipefail\ndeno install \\\n--allow-read --allow-write \\\n--allow-env --allow-net --allow-run \\\n--no-check \\\n-r -f https://deno.land/x/deploy/deployctl.ts\n\n", script)

	script = prepareScriptFromCommands([]string{
		`pipenv run bash -c 'echo "Some message"'`,
	}, "bash")
	assert.Equal(t, "set -e -o pipefail\npipenv run bash -c 'echo \"Some message\"'\n\n", script)

	script = prepareScriptFromCommands([]string{
		`brew bundle --no-lock`,
		`brew upgrade`,
	}, "sh")
	assert.Equal(t, "set -e\nbrew bundle --no-lock\nbrew upgrade\n\n", script)

	script = prepareScriptFromCommands([]string{
		`brew bundle --no-lock`,
		`brew upgrade`,
	}, "pwsh")
	assert.Equal(t, "\nbrew bundle --no-lock\nbrew upgrade\n\n", script)
}

func TestShellFromShellPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		assert.Equal(t,
			"pwsh",
			ShellFromShellPath("c:\\System32\\pwsh.exe"),
		)
	} else {
		assert.Equal(t,
			"sh",
			ShellFromShellPath("/bin/sh"),
		)

		assert.Equal(t,
			"bash",
			ShellFromShellPath("/bin/bash"),
		)

		assert.Equal(t,
			"zsh",
			ShellFromShellPath("/bin/zsh"),
		)
	}
}

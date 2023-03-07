package runner

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrepareScript(t *testing.T) {
	script := prepareScriptFromCommands([]string{
		`# macOS`,
		`brew bundle --no-lock`,
		`brew upgrade`,
	}, "bash")
	assert.Equal(t, "set -e -o pipefail;brew bundle --no-lock;brew upgrade;\n", script)

	script = prepareScriptFromCommands([]string{
		"deno install \\",
		"--allow-read --allow-write \\",
		"--allow-env --allow-net --allow-run \\",
		"--no-check \\",
		"-r -f https://deno.land/x/deploy/deployctl.ts",
	}, "bash")
	assert.Equal(t, "set -e -o pipefail;deno install --allow-read --allow-write --allow-env --allow-net --allow-run --no-check -r -f https://deno.land/x/deploy/deployctl.ts;\n", script)

	script = prepareScriptFromCommands([]string{
		`pipenv run bash -c 'echo "Some message"'`,
	}, "bash")
	assert.Equal(t, "set -e -o pipefail;pipenv run bash -c \"echo \\\"Some message\\\"\";\n", script)

	script = prepareScriptFromCommands([]string{
		`brew bundle --no-lock`,
		`brew upgrade`,
	}, "sh")
	assert.Equal(t, "set -e;brew bundle --no-lock;brew upgrade;\n", script)

	script = prepareScriptFromCommands([]string{
		`brew bundle --no-lock`,
		`brew upgrade`,
	}, "pwsh")
	assert.Equal(t, "brew bundle --no-lock;brew upgrade;\n", script)
}

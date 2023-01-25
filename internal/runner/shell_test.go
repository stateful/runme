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
	})
	assert.Equal(t, "set -e -o pipefail;brew bundle --no-lock;brew upgrade;\n", script)

	script = prepareScriptFromCommands([]string{
		"deno install \\",
		"--allow-read --allow-write \\",
		"--allow-env --allow-net --allow-run \\",
		"--no-check \\",
		"-r -f https://deno.land/x/deploy/deployctl.ts",
	})
	assert.Equal(t, "set -e -o pipefail;deno install --allow-read --allow-write --allow-env --allow-net --allow-run --no-check -r -f https://deno.land/x/deploy/deployctl.ts;\n", script)

	script = prepareScriptFromCommands([]string{
		`pipenv run bash -c 'echo "Some message"'`,
	})
	assert.Equal(t, "set -e -o pipefail;pipenv run bash -c \"echo \\\"Some message\\\"\";\n", script)
}

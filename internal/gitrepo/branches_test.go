package gitrepo

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var branchFixtures = `
Merge pull request #333 from stateful:seb/cal-edits--||--More text edits etc
Merge pull request #220 from stateful/admc/status-vscode-button--||--Add open in vscode button
Merge pull request #132 from stateful/jgee/feat/cli-instructions-platform-specific--||--Use accordion like component to contain CLI instructions
Merge branch 'main' into jgee/feat/cli-instructions-platform-specific--||--
Merge branch 'main' into admc/standup-ux-refactor--||--
Merging--||--
Merge branch 'admc/slack-attack'--||--
Merge pull request #7 from activecove/seb/file-cycles--||--Move to file sessions`

func Test_getBranchNamesFromStdout(t *testing.T) {
	t.Run("NonGreedy", func(t *testing.T) {
		expected := []Branch{
			{Name: "seb/cal-edits", Description: "More text edits etc"},
			{Name: "admc/status-vscode-button", Description: "Add open in vscode button"},
			{Name: "jgee/feat/cli-instructions-platform-specific", Description: "Use accordion like component to contain CLI instructions"},
			{Name: "seb/file-cycles", Description: "Move to file sessions"},
		}
		actual := getBranchNamesFromStdout(branchFixtures, false)
		require.Equal(t, expected, actual)
	})

	t.Run("Greedy", func(t *testing.T) {
		expected := []Branch{
			{Name: "cal-edits", Description: "More text edits etc"},
			{Name: "status-vscode-button", Description: "Add open in vscode button"},
			{Name: "cli-instructions-platform-specific", Description: "Use accordion like component to contain CLI instructions"},
			{Name: "file-cycles", Description: "Move to file sessions"},
		}
		actual := getBranchNamesFromStdout(branchFixtures, true)
		require.Equal(t, expected, actual)
	})
}

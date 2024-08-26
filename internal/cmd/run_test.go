package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stateful/runme/v3/internal/runner/client"
	"github.com/stateful/runme/v3/pkg/project"
	"github.com/stretchr/testify/assert"
)

func TestPromptEnvVars(t *testing.T) {
	var (
		getRunnerOpts func() ([]client.RunnerOption, error)
		serverAddr    string
	)

	RootCmd := &cobra.Command{
		Use:           "root",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	cmd := &cobra.Command{
		Use:               "run",
		Args:              cobra.ArbitraryArgs,
		ValidArgsFunction: validCmdNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, err := getProject()
			assert.NoError(t, err)

			runnerOpts, err := getRunnerOpts()
			assert.NoError(t, err)

			var stdin io.Reader
			stdin = bytes.NewBuffer([]byte{})
			if isTerminal(os.Stdout.Fd()) {
				stdin = cmd.InOrStdin()
			}

			runnerOpts = append(
				runnerOpts,
				client.WithInsecure(true),
				client.WithinShellMaybe(),
				client.WithStdin(stdin),
				client.WithStdout(cmd.OutOrStdout()),
				client.WithStderr(cmd.ErrOrStderr()),
				client.WithProject(proj),
			)

			runner, err := client.NewLocalRunner(runnerOpts...)
			assert.NoError(t, err)
			tasks, err := getProjectTasks(cmd)
			assert.NoError(t, err)

			runTasks := []project.Task{}
			for _, arg := range args {
				task, err := lookupTaskWithPrompt(cmd, arg, tasks)
				assert.NoError(t, err)

				runTasks = append(runTasks, task)
			}

			err = promptEnvVars(cmd, runner, runTasks...)
			assert.NoError(t, err)

			expectedLines := `#
# VAR_NAME1 set in managed env store
# "export VAR_NAME1='Placeholder 1'"
#
# VAR_NAME2 set in managed env store
# "export VAR_NAME2=\"Placeholder 2\""
#
# VAR_NAME3 set in managed env store
# "export VAR_NAME3=\"\""
#
# VAR_NAME4 set in managed env store
# "export VAR_NAME4=Message"


echo "1. ${VAR_NAME1}"
echo "2. ${VAR_NAME2}"
echo "3. ${VAR_NAME3}"
echo "4. ${VAR_NAME4}"
`
			lines := runTasks[0].CodeBlock.Lines()
			resultLines := strings.Join(lines, "\n")

			assert.Equal(t, expectedLines, resultLines)

			return nil
		},
	}
	getRunnerOpts = setRunnerFlags(cmd, &serverAddr)

	actual := new(bytes.Buffer)
	RootCmd.SetOut(actual)
	RootCmd.SetErr(actual)
	RootCmd.SetArgs([]string{"run", "vars"})
	RootCmd.AddCommand(cmd)

	err := RootCmd.Execute()
	assert.NoError(t, err)
}

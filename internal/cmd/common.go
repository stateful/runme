package cmd

import (
	"strings"

	"github.com/spf13/cobra"
)

func validCmdNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	p, err := newParser()
	if err != nil {
		cmd.PrintErrf("failed to get parser: %s", err)
		return nil, cobra.ShellCompDirectiveError
	}

	names := p.Snippets().GetNames()

	var filtered []string
	for _, name := range names {
		if strings.HasPrefix(name, toComplete) {
			filtered = append(filtered, name)
		}
	}

	return filtered, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
}

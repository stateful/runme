package cmd

import (
	"io"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/stateful/runme/internal/document"
	"github.com/stateful/runme/internal/renderer/md"
	"github.com/stateful/runme/internal/runner"
)

func getCodeBlocks() (document.CodeBlocks, error) {
	f, err := os.DirFS(chdir).Open(fileName)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	doc := document.New(data, md.Render)
	node, err := doc.Parse()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	blocks := document.CollectCodeBlocks(node)

	filtered := make(document.CodeBlocks, 0, len(blocks))
	for _, b := range blocks {
		if allowUnknown || (b.Language() != "" && runner.IsSupported(b.Language())) {
			filtered = append(filtered, b)
		}
	}
	return filtered, nil
}

func lookupCodeBlock(blocks document.CodeBlocks, name string) (*document.CodeBlock, error) {
	block := blocks.Lookup(name)
	if block == nil {
		return nil, errors.Errorf("command %q not found; known command names: %s", name, blocks.Names())
	}
	return block, nil
}

func validCmdNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	blocks, err := getCodeBlocks()
	if err != nil {
		cmd.PrintErrf("failed to get parser: %s", err)
		return nil, cobra.ShellCompDirectiveError
	}

	names := blocks.Names()

	var filtered []string
	for _, name := range names {
		if strings.HasPrefix(name, toComplete) {
			filtered = append(filtered, name)
		}
	}
	return filtered, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
}

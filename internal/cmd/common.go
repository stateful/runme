package cmd

import (
	"io"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/stateful/runme/internal/document"
	"github.com/stateful/runme/internal/renderer"
)

func getCodeBlocks() (document.CodeBlocks, error) {
	source, err := document.NewSourceFromFile(os.DirFS(chdir), fileName)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return source.Parse().CodeBlocks(), nil
}

func lookupCodeBlock(blocks document.CodeBlocks, name string) (*document.CodeBlock, error) {
	block := blocks.Lookup(name)
	if block == nil {
		return nil, errors.Errorf("command %q not found; known command names: %s", name, blocks.Names())
	}
	return block, nil
}

func renderToJSON(w io.Writer) error {
	source, err := document.NewSourceFromFile(os.DirFS(chdir), fileName)
	if err != nil {
		return errors.WithStack(err)
	}

	parsed := source.Parse()
	sourceData, root := parsed.Source(), parsed.Root()

	err = renderer.RenderToJSON(w, sourceData, root)
	return errors.WithMessage(err, "failed to render to JSON")
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

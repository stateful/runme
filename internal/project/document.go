package project

import (
	"io"
	"os"

	"github.com/pkg/errors"
	"github.com/stateful/runme/internal/document"
	"github.com/stateful/runme/internal/renderer/cmark"
	"github.com/stateful/runme/internal/runner"
)

func ReadMarkdownFile(filepath string) ([]byte, error) {
	f, err := os.Open(filepath)

	if err != nil {
		var pathError *os.PathError
		if errors.As(err, &pathError) {
			return nil, errors.Errorf("failed to %s markdown file %s: %s", pathError.Op, pathError.Path, pathError.Err.Error())
		}

		return nil, errors.Wrapf(err, "failed to read %s", filepath)
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read data")
	}
	return data, nil
}

func GetCodeBlocks(filepath string, allowUnknown bool) (document.CodeBlocks, error) {
	data, err := ReadMarkdownFile(filepath)
	if err != nil {
		return nil, err
	}

	doc := document.New(data, cmark.Render)
	node, _, err := doc.Parse()
	if err != nil {
		return nil, err
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

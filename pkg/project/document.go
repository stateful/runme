package project

import (
	"io"
	"os"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-billy/v5/util"
	"github.com/pkg/errors"
	"github.com/stateful/runme/v3/internal/document"
	"github.com/stateful/runme/v3/internal/document/identity"
)

func ReadMarkdownFile(filepath string, fs billy.Basic) ([]byte, error) {
	if fs == nil {
		fs = osfs.Default
	}

	f, err := fs.Open(filepath)
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

func WriteMarkdownFile(filename string, fs billy.Basic, data []byte) error {
	if fs == nil {
		fs = osfs.Default
	}

	return util.WriteFile(fs, filename, data, 0o600)
}

func parseDocumentForCodeBlocks(filepath string, fs billy.Basic, doFrontmatter bool) (document.CodeBlocks, *document.Frontmatter, error) {
	data, err := ReadMarkdownFile(filepath, fs)
	if err != nil {
		return nil, nil, err
	}

	var fmtr *document.Frontmatter

	if doFrontmatter {
		sections, err := document.ParseSections(data)
		if err != nil {
			return nil, nil, err
		}

		f, _ := document.ParseFrontmatter(sections.FrontMatter)
		fmtr = f
	}

	identityResolver := identity.NewResolver(identity.DefaultLifecycleIdentity)
	doc := document.New(data, identityResolver)
	node, err := doc.Root()
	if err != nil {
		return nil, nil, err
	}

	blocks := document.CollectCodeBlocks(node)

	return blocks, fmtr, nil
}

func GetCodeBlocksAndParseFrontmatter(filepath string, fs billy.Basic) (document.CodeBlocks, document.Frontmatter, error) {
	blocks, fmtr, err := parseDocumentForCodeBlocks(filepath, fs, true)

	var f document.Frontmatter
	if fmtr != nil {
		f = *fmtr
	}

	return blocks, f, err
}

func GetCodeBlocks(filepath string, fs billy.Basic) (document.CodeBlocks, error) {
	blocks, _, err := parseDocumentForCodeBlocks(filepath, fs, false)
	return blocks, err
}

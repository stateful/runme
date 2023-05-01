package project

import (
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/stateful/runme/internal/document"
	"github.com/stateful/runme/internal/renderer/cmark"
	"github.com/stateful/runme/internal/runner"
)

type Project struct {
	RootDir    string
	BranchName string
	Commit     string
	URL        string
}

func New(cwd string) (p Project) {
	r := &Resolver{cwd: cwd}
	p.RootDir = r.RootDir()
	return p
}

func NetGit(cwd string) (Project, error) {
	p := New(cwd)
	r := &Resolver{cwd: cwd}
	p.RootDir = r.RootDir()
	return r.Get()
}

func (p *Project) getAllMarkdownFiles() ([]string, error) {
	root, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	var files []string
	err = filepath.WalkDir(root, func(s string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if filepath.Ext(d.Name()) == ".md" {
			files = append(files, s)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}

func (p *Project) ReadMarkdownFile(relativePath string, args []string) ([]byte, error) {
	absPath := path.Join(p.RootDir, relativePath)
	rootDir := path.Dir(absPath)
	fileName := path.Base(absPath)

	arg := ""
	if len(args) == 1 {
		arg = args[0]
	}

	if arg == "" {
		f, err := os.DirFS(rootDir).Open(fileName)
		if err != nil {
			var pathError *os.PathError
			if errors.As(err, &pathError) {
				return nil, errors.Errorf("failed to %s markdown file %s: %s", pathError.Op, pathError.Path, pathError.Err.Error())
			}

			return nil, errors.Wrapf(err, "failed to read %s", filepath.Join(p.RootDir, fileName))
		}
		defer func() { _ = f.Close() }()
		data, err := io.ReadAll(f)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read data")
		}
		return data, nil
	}

	var (
		data []byte
		err  error
	)

	if arg == "-" {
		data, err = io.ReadAll(os.Stdin)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read from stdin")
		}
	} else if strings.HasPrefix(arg, "https://") {
		client := http.Client{
			Timeout: time.Second * 5,
		}
		resp, err := client.Get(arg)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get a file %q", arg)
		}
		defer func() { _ = resp.Body.Close() }()
		data, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read body")
		}
	} else {
		f, err := os.Open(arg)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to open file %q", arg)
		}
		defer func() { _ = f.Close() }()
		data, err = io.ReadAll(f)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read from file %q", arg)
		}
	}

	return data, nil
}

func (p *Project) GetCodeBlocks(relativePath string, allowUnknown bool) (document.CodeBlocks, error) {
	data, err := p.ReadMarkdownFile(relativePath, nil)
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

type FileCodeBlocks struct {
	FileName   string
	CodeBlocks document.CodeBlocks
}
type CodeBlocks []*FileCodeBlocks

func (p *Project) GetAllCodeBlocks(allowUnknown bool) (CodeBlocks, error) {
	files, err := p.getAllMarkdownFiles()
	if err != nil {
		return nil, err
	}

	blocks := CodeBlocks{}
	for _, file := range files {
		codeBlock, err := p.GetCodeBlocks(file[len(p.RootDir):], allowUnknown)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, &FileCodeBlocks{
			FileName:   file[len(p.RootDir):],
			CodeBlocks: codeBlock,
		})
	}

	return blocks, nil
}

func (p *Project) LookUpCodeBlockByID(id string) (*string, *document.CodeBlock, error) {
	files, err := p.getAllMarkdownFiles()
	if err != nil {
		return nil, nil, err
	}

	for _, file := range files {
		codeBlock, err := p.GetCodeBlocks(file[len(p.RootDir):], false)
		if err != nil {
			return nil, nil, err
		}
		for _, block := range codeBlock {
			if block.Name() == id {
				relativeFilePath := file[len(p.RootDir):]
				return &relativeFilePath, block, nil
			}
		}
	}

	return nil, nil, errors.Errorf("No code block found with id %s", id)
}

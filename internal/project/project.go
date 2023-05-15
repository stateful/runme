package project

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-billy/v5/util"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	"github.com/stateful/runme/internal/document"
)

type CodeBlock struct {
	Block *document.CodeBlock
	File  string
}

func (b CodeBlock) GetBlock() *document.CodeBlock {
	return b.Block
}

func (b CodeBlock) GetFile() string {
	return b.File
}

type FileCodeBlock interface {
	GetBlock() *document.CodeBlock
	GetFile() string
}

type CodeBlocks []CodeBlock

func (blocks CodeBlocks) Lookup(queryName string) []CodeBlock {
	results := make([]CodeBlock, 0)

	for _, block := range blocks {
		if queryName != block.Block.Name() {
			continue
		}

		results = append(results, block)
	}

	return results
}

type ErrCodeBlockFileNotFound struct {
	queryFile string
}

func (e ErrCodeBlockFileNotFound) Error() string {
	return fmt.Sprintf("unable to find file in project matching regex %q", e.FailedFileQuery())
}

func (e ErrCodeBlockFileNotFound) FailedFileQuery() string {
	return e.queryFile
}

type ErrCodeBlockNameNotFound struct {
	queryName string
}

func (e ErrCodeBlockNameNotFound) Error() string {
	return fmt.Sprintf("unable to find any script named %q", e.queryName)
}

func (e ErrCodeBlockNameNotFound) FailedNameQuery() string {
	return e.queryName
}

func (blocks CodeBlocks) LookupWithFile(queryFile string, queryName string) ([]CodeBlock, error) {
	var queryMatcher *regexp.Regexp
	if queryFile != "" {
		reg, err := regexp.Compile(queryFile)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid regexp %s", queryFile)
		}
		queryMatcher = reg
	}

	results := make([]CodeBlock, 0)

	foundFile := false

	for _, block := range blocks {
		if queryMatcher != nil && !queryMatcher.MatchString(block.File) {
			continue
		}

		foundFile = true

		if queryName != block.Block.Name() {
			continue
		}

		results = append(results, block)
	}

	if len(results) == 0 {
		if !foundFile {
			return nil, ErrCodeBlockFileNotFound{queryFile: queryFile}
		} else {
			return nil, ErrCodeBlockNameNotFound{queryName: queryName}
		}
	}

	return results, nil
}

func (blocks CodeBlocks) Names() []string {
	return nil
}

type Project interface {
	LoadTasks() (CodeBlocks, error)
	LoadEnvs() (map[string]string, error)
}

type DirectoryProject struct {
	repo *git.Repository
	fs   billy.Filesystem

	allowUnknown bool
	envLoadOrder []string
}

// TODO(mxs): support `.runmeignore` file
type DirectoryProjectMatcher struct {
	gitMatcher gitignore.Matcher
}

func (m *DirectoryProjectMatcher) Match(path []string, isDir bool) bool {
	if m.gitMatcher != nil && m.gitMatcher.Match(path, isDir) {
		return true
	}

	return false
}

func (p *DirectoryProject) SetEnvLoadOrder(envLoadOrder []string) {
	p.envLoadOrder = envLoadOrder
}

func NewDirectoryProject(dir string, findNearestRepo bool, allowUnknown bool) (*DirectoryProject, error) {
	project := &DirectoryProject{
		allowUnknown: allowUnknown,
		envLoadOrder: []string{
			".env.local",
			".env",
		},
	}

	// try to find nearest git repo
	if findNearestRepo {
		repo, err := git.PlainOpenWithOptions(dir, &git.PlainOpenOptions{
			DetectDotGit: true,
		})
		if err != nil && !errors.Is(err, git.ErrRepositoryNotExists) {
			return nil, err
		}

		if repo != nil {
			if wt, err := repo.Worktree(); err == nil {
				project.fs = wt.Filesystem
			}

			project.repo = repo
		}
	}

	if project.fs == nil {
		project.fs = osfs.New(dir)
	}

	return project, nil
}

func (p *DirectoryProject) LoadTasks() (CodeBlocks, error) {
	matcher := &DirectoryProjectMatcher{}

	if p.repo != nil {
		ps, _ := gitignore.ReadPatterns(p.fs, []string{})
		dotGitPs := gitignore.ParsePattern("/.git", []string{})

		matcher.gitMatcher = gitignore.NewMatcher(append([]gitignore.Pattern{dotGitPs}, ps...))
	}

	type RepoWalkNode struct {
		path string
		info fs.FileInfo
	}

	rootInfo, err := p.fs.Stat(".")
	if err != nil {
		return nil, err
	}

	stk := []RepoWalkNode{{
		path: ".",
		info: rootInfo,
	}}

	markdownFiles := make([]string, 0)

	for len(stk) > 0 {
		var node RepoWalkNode
		stk, node = stk[:len(stk)-1], stk[len(stk)-1]

		if node.info.IsDir() {
			info, err := p.fs.ReadDir(node.path)
			if err != nil {
				return nil, err
			}

			for _, subfile := range info {
				subfilePath := p.fs.Join(node.path, subfile.Name())

				if matcher.Match(
					strings.Split(subfilePath, string(filepath.Separator)),
					subfile.IsDir(),
				) {
					continue
				}

				stk = append(stk, RepoWalkNode{
					path: filepath.Join(node.path, subfile.Name()),
					info: subfile,
				})
			}
		} else {
			ext := strings.ToLower(filepath.Ext(node.path))

			if ext == ".md" || ext == ".mdx" || ext == ".mdi" {
				markdownFiles = append(markdownFiles, node.path)
			}
		}
	}

	result := make(CodeBlocks, 0)

	for _, mdFile := range markdownFiles {
		blocks, err := getFileCodeBlocks(mdFile, p.allowUnknown, p.fs)
		if err != nil {
			return nil, err
		}

		result = append(result, blocks...)
	}

	return result, nil
}

func (p *DirectoryProject) LoadEnvs() (map[string]string, error) {
	envs := make(map[string]string)

	for _, envFile := range p.envLoadOrder {
		bytes, err := util.ReadFile(p.fs, envFile)
		var pathError *os.PathError
		if err != nil {
			if !errors.As(err, &pathError) {
				return nil, err
			}

			continue
		}

		parsed, err := godotenv.UnmarshalBytes(bytes)
		if err != nil {
			// silently fail for now
			// TODO(mxs): come up with better solution
			continue
		}

		for k, v := range parsed {
			envs[k] = v
		}
	}

	return envs, nil
}

type SingleFileProject struct {
	file         string
	allowUnknown bool
}

func NewSingleFileProject(file string, allowUnknown bool) *SingleFileProject {
	return &SingleFileProject{
		file:         file,
		allowUnknown: allowUnknown,
	}
}

func (p *SingleFileProject) LoadTasks() (CodeBlocks, error) {
	fs := osfs.New(filepath.Dir(p.file))
	relFile, err := filepath.Rel(fs.Root(), p.file)
	if err != nil {
		return nil, err
	}

	return getFileCodeBlocks(relFile, p.allowUnknown, fs)
}

func (p *SingleFileProject) LoadEnvs() (map[string]string, error) {
	return nil, nil
}

func getFileCodeBlocks(file string, allowUnknown bool, fs billy.Basic) ([]CodeBlock, error) {
	blocks, err := GetCodeBlocks(file, allowUnknown, fs)
	if err != nil {
		return nil, err
	}

	fileBlocks := make(CodeBlocks, len(blocks))

	for i, block := range blocks {
		fileBlocks[i] = CodeBlock{
			File:  file,
			Block: block,
		}
	}

	return fileBlocks, nil
}

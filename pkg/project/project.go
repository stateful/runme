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
	"github.com/stateful/runme/v3/internal/document"
)

type CodeBlock struct {
	Block *document.CodeBlock
	/// Relative to `project.Root()`
	File        string
	Frontmatter document.Frontmatter
	fs          billy.Chroot
}

func newCodeBlock(
	block *document.CodeBlock,
	file string,
	frontmatter document.Frontmatter,
	fs billy.Chroot,
) *CodeBlock {
	return &CodeBlock{
		Block:       block,
		File:        file,
		Frontmatter: frontmatter,
		fs:          fs,
	}
}

func (b CodeBlock) GetBlock() *document.CodeBlock {
	return b.Block
}

func (b CodeBlock) Clone() *CodeBlock {
	block := b.Block.Clone()
	return newCodeBlock(
		block,
		b.File,
		b.Frontmatter,
		b.fs,
	)
}

func (b CodeBlock) GetFileRel() string {
	return b.File
}

func (b CodeBlock) GetFile() string {
	return filepath.Join(b.fs.Root(), b.File)
}

func (b CodeBlock) GetID() string {
	return fmt.Sprintf("%s:%s", b.File, b.Block.Name())
}

func (b CodeBlock) GetFrontmatter() document.Frontmatter {
	return b.Frontmatter
}

type FileCodeBlock interface {
	GetBlock() *document.CodeBlock

	// relative to project root
	GetFileRel() string

	// absolute file path
	GetFile() string
	GetFrontmatter() document.Frontmatter
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

func IsCodeBlockNotFoundError(err error) bool {
	return errors.As(err, &ErrCodeBlockNameNotFound{}) || errors.As(err, &ErrCodeBlockFileNotFound{})
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

func (blocks CodeBlocks) getFileRegexp(query string) (*regexp.Regexp, error) {
	if query != "" {
		reg, err := regexp.Compile(query)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid regexp %s", query)
		}
		return reg, nil
	}

	return nil, nil
}

func (blocks CodeBlocks) LookupByID(query string) ([]CodeBlock, error) {
	queryMatcher, err := blocks.getFileRegexp(query)
	if err != nil {
		return nil, err
	}

	results := make([]CodeBlock, 0)

	for _, block := range blocks {
		if queryMatcher != nil && !queryMatcher.MatchString(block.GetID()) {
			continue
		}

		results = append(results, block)
	}

	return results, nil
}

func (blocks CodeBlocks) LookupByFile(queryFile string) ([]CodeBlock, error) {
	queryMatcher, err := blocks.getFileRegexp(queryFile)
	if err != nil {
		return nil, err
	}

	results := make([]CodeBlock, 0)

	for _, block := range blocks {
		if queryMatcher != nil && !queryMatcher.MatchString(block.File) {
			continue
		}

		results = append(results, block)
	}

	return results, nil
}

func (blocks CodeBlocks) LookupWithFile(queryFile string, queryName string) ([]CodeBlock, error) {
	queryMatcher, err := blocks.getFileRegexp(queryFile)
	if err != nil {
		return nil, err
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
		if !foundFile && queryFile != "" {
			return nil, ErrCodeBlockFileNotFound{queryFile: queryFile}
		}

		return nil, ErrCodeBlockNameNotFound{queryName: queryName}
	}

	return results, nil
}

func (blocks CodeBlocks) Names() []string {
	return nil
}

type (
	LoadTaskStatusSearchingFiles struct{}
	LoadTaskStatusParsingFiles   struct{}
)

type LoadTaskSearchingFolder struct {
	Folder string
}

type LoadTaskParsingFile struct {
	Filename string
}

type LoadTaskFoundFile struct {
	Filename string
}

type LoadTaskFoundTask struct {
	Task CodeBlock
}

type LoadTaskError struct {
	Err error
}

type Project interface {
	// Loads tasks in project, sending details to provided channel. Will block, but is thread-safe.
	//
	// Received messages for the channel will be of type `project.LoadTask*`. The
	// channel will be closed on finish or error.
	//
	// Use `filesOnly` to just find files, skipping markdown parsing
	LoadTasks(filesOnly bool, channel chan<- interface{})
	LoadEnvs() (map[string]string, error)
	EnvLoadOrder() []string
	Dir() string
}

type DirectoryProject struct {
	repo *git.Repository
	fs   billy.Filesystem

	respectGitignore bool
	envLoadOrder     []string

	ignorePatterns []string
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

func (p *DirectoryProject) SetRespectGitignore(respectGitignore bool) {
	p.respectGitignore = respectGitignore
}

func NewDirectoryProject(dir string, findNearestRepo bool, allowUnknown bool, allowUnnamed bool, ignorePatterns []string) (*DirectoryProject, error) {
	project := &DirectoryProject{
		respectGitignore: true,
		ignorePatterns:   ignorePatterns,
	}

	// try to find git repo
	{
		var (
			repo *git.Repository
			err  error
		)

		if findNearestRepo {
			repo, err = git.PlainOpenWithOptions(dir, &git.PlainOpenOptions{
				DetectDotGit: true,
			})
		} else {
			repo, err = git.PlainOpen(dir)
		}

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

func (p *DirectoryProject) LoadTasks(filesOnly bool, channel chan<- interface{}) {
	defer close(channel)

	matcher := &DirectoryProjectMatcher{}

	ignores := []gitignore.Pattern{}

	if p.repo != nil && p.respectGitignore {
		ps, _ := gitignore.ReadPatterns(p.fs, []string{})
		dotGitPs := gitignore.ParsePattern("/.git", []string{})

		ignores = append(ignores, dotGitPs)
		ignores = append(ignores, ps...)
	}

	for _, pattern := range p.ignorePatterns {
		ignores = append(ignores, gitignore.ParsePattern(pattern, []string{}))
	}

	matcher.gitMatcher = gitignore.NewMatcher(ignores)

	type RepoWalkNode struct {
		path string
		info fs.FileInfo
	}

	rootInfo, err := p.fs.Stat(".")
	if err != nil {
		channel <- LoadTaskError{Err: err}
		return
	}

	stk := []RepoWalkNode{{
		path: ".",
		info: rootInfo,
	}}

	runbookFiles := make([]string, 0)

	channel <- LoadTaskStatusSearchingFiles{}

	for len(stk) > 0 {
		var node RepoWalkNode
		stk, node = stk[:len(stk)-1], stk[len(stk)-1]

		if node.info.IsDir() {
			channel <- LoadTaskSearchingFolder{Folder: node.path}

			info, err := p.fs.ReadDir(node.path)
			if err != nil {
				channel <- LoadTaskError{Err: err}
				return
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

			if ext == ".md" || ext == ".mdx" || ext == ".mdi" || ext == ".mdr" || ext == ".run" || ext == ".runme" {
				channel <- LoadTaskFoundFile{Filename: node.path}

				if !filesOnly {
					runbookFiles = append(runbookFiles, node.path)
				}
			}
		}
	}

	if filesOnly {
		return
	}

	channel <- LoadTaskStatusParsingFiles{}

	for _, runFile := range runbookFiles {
		channel <- LoadTaskParsingFile{Filename: runFile}
		blocks, err := getFileCodeBlocks(runFile, p.fs)
		if err != nil {
			channel <- LoadTaskError{Err: err}
			return
		}

		for _, block := range blocks {
			channel <- LoadTaskFoundTask{Task: block}
		}
	}
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

		parsed, comments, err := godotenv.UnmarshalBytesWithComments(bytes)
		if err != nil {
			// silently fail for now
			// TODO(mxs): come up with better solution
			continue
		}

		//revive:disable:unhandled-error
		fmt.Printf("%v\n", comments)

		for k, v := range parsed {
			envs[k] = v
		}
	}

	return envs, nil
}

func (p *DirectoryProject) EnvLoadOrder() []string {
	return p.envLoadOrder
}

func (p *DirectoryProject) Dir() string {
	return p.fs.Root()
}

type SingleFileProject struct {
	file         string
	allowUnknown bool
	allowUnnamed bool
}

func NewSingleFileProject(file string, allowUnknown bool, allowUnnamed bool) *SingleFileProject {
	return &SingleFileProject{
		file:         file,
		allowUnknown: allowUnknown,
		allowUnnamed: allowUnnamed,
	}
}

func (p *SingleFileProject) LoadTasks(filesOnly bool, channel chan<- interface{}) {
	defer close(channel)

	channel <- LoadTaskStatusSearchingFiles{}

	fs := osfs.New(p.Dir())
	channel <- LoadTaskSearchingFolder{Folder: "."}

	relFile, err := filepath.Rel(fs.Root(), p.file)
	if err != nil {
		channel <- LoadTaskError{Err: err}
		return
	}
	channel <- LoadTaskFoundFile{Filename: relFile}

	channel <- LoadTaskStatusParsingFiles{}

	channel <- LoadTaskParsingFile{Filename: relFile}
	blocks, err := getFileCodeBlocks(relFile, fs)
	if err != nil {
		channel <- LoadTaskError{Err: err}
		return
	}

	for _, block := range blocks {
		channel <- LoadTaskFoundTask{Task: block}
	}
}

func (p *SingleFileProject) LoadEnvs() (map[string]string, error) {
	return nil, nil
}

func (p *SingleFileProject) EnvLoadOrder() []string {
	return nil
}

func (p *SingleFileProject) Dir() string {
	return filepath.Dir(p.file)
}

type CodeBlockFS interface {
	billy.Basic
	billy.Chroot
}

func getFileCodeBlocks(file string, fs CodeBlockFS) ([]CodeBlock, error) {
	blocks, fmtr, err := GetCodeBlocksAndParseFrontmatter(file, fs)
	if err != nil {
		return nil, err
	}

	fileBlocks := make(CodeBlocks, len(blocks))

	for i, block := range blocks {
		fileBlocks[i] = *newCodeBlock(
			block, file, fmtr, fs,
		)
	}

	return fileBlocks, nil
}

// Load tasks, blocking until all projects are loaded
func LoadProjectTasks(proj Project) (CodeBlocks, error) {
	channel := make(chan interface{})
	go proj.LoadTasks(false, channel)

	blocks := make(CodeBlocks, 0)
	var err error

	for raw := range channel {
		switch msg := raw.(type) {
		case LoadTaskError:
			err = msg.Err
		case LoadTaskFoundTask:
			blocks = append(blocks, msg.Task)
		}
	}

	return blocks, err
}

// Load files, blocking until all projects are loaded
func LoadProjectFiles(proj Project) ([]string, error) {
	channel := make(chan interface{})
	go proj.LoadTasks(true, channel)

	files := make([]string, 0)
	var err error

	for raw := range channel {
		switch msg := raw.(type) {
		case LoadTaskError:
			err = msg.Err
		case LoadTaskFoundFile:
			files = append(files, msg.Filename)
		}
	}

	return files, err
}

func FilterCodeBlocks[T FileCodeBlock](blocks []T, allowUnknown bool, allowUnnamed bool) (result []T) {
	for _, b := range blocks {
		if !allowUnknown && b.GetBlock().IsUnknown() {
			continue
		}

		if !allowUnnamed && b.GetBlock().IsUnnamed() {
			continue
		}

		result = append(result, b)
	}

	return
}

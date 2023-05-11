package project

import (
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/gobwas/glob"
	"github.com/pkg/errors"
	"github.com/stateful/runme/internal/document"
)

const fileNameSeparator = ":"

type CodeBlock struct {
	Block *document.CodeBlock
	File  string
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

func (blocks CodeBlocks) LookupWithFile(queryFile string, queryName string) ([]CodeBlock, error) {
	// parts := strings.SplitN(name, fileNameSeparator, 2)

	// var queryFile string
	// var queryName string

	// if len(parts) > 1 {
	// 	queryFile := parts[0]
	// 	queryName = parts[1]
	// } else {
	// 	queryName = name
	// }

	var queryMatcher glob.Glob
	if queryFile != "" {
		glob, err := glob.Compile(queryFile, '/', '\\')
		if err != nil {
			return nil, errors.Wrapf(err, "invalid glob sequence %s", queryFile)
		}
		queryMatcher = glob
	}

	results := make([]CodeBlock, 0)

	for _, block := range blocks {
		if queryMatcher != nil && !queryMatcher.Match(block.File) {
			continue
		}

		if queryName != block.Block.Name() {
			continue
		}

		results = append(results, block)
	}

	return results, nil
}

func (blocks CodeBlocks) Names() []string {
	return nil
}

type Project interface {
	LoadTasks() CodeBlocks
	LoadEnvs() []string
}

type DirectoryProject struct {
	repo *git.Repository
	fs   billy.Filesystem

	allowUnknown bool
}

func NewDirectoryProject(dir string, findNearestRepo bool, allowUnknown bool) (*DirectoryProject, error) {
	// TODO: find closest git repo
	// util.Walk()

	project := &DirectoryProject{
		allowUnknown: allowUnknown,
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
			if wt, err := repo.Worktree(); err != nil {
				project.fs = wt.Filesystem
			}
		}
	}

	if project.fs == nil {
		project.fs = osfs.New(dir)
	}

	return project, nil
}

func (p *DirectoryProject) LoadTasks() ([]CodeBlock, error) {

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

func (p *SingleFileProject) LoadTasks() ([]CodeBlock, error) {
	blocks, err := GetCodeBlocks(p.file, p.allowUnknown)
	if err != nil {
		return nil, err
	}

	fileBlocks := make(CodeBlocks, len(blocks))

	for _, block := range blocks {
		fileBlocks = append(fileBlocks, CodeBlock{
			File:  p.file,
			Block: block,
		})
	}

	return fileBlocks, nil
}

func (p *SingleFileProject) LoadEnvs() []string {
	return nil
}

func getFileCodeBlocks() {

}

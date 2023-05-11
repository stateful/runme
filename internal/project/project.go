package project

import "github.com/stateful/runme/internal/document"

type CodeBlock struct {
	Block *document.CodeBlock
	File  string
}

type CodeBlocks = []CodeBlock

type Project interface {
	LoadTasks() CodeBlocks
	LoadEnvs() []string
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

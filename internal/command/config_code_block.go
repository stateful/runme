package command

import (
	"os"
	"path/filepath"
	"strings"

	runnerv2 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/runner/v2"
	"github.com/stateful/runme/v3/pkg/document"
)

type ConfigBuilderOption func(*configBuilder) error

func WithInteractiveLegacy() ConfigBuilderOption {
	return func(b *configBuilder) error {
		b.useInteractiveLegacy = true
		return nil
	}
}

func NewProgramConfigFromCodeBlock(block *document.CodeBlock, opts ...ConfigBuilderOption) (*ProgramConfig, error) {
	b := &configBuilder{block: block}

	for _, opt := range opts {
		if err := opt(b); err != nil {
			return nil, err
		}
	}

	return b.Build()
}

type configBuilder struct {
	block                *document.CodeBlock
	useInteractiveLegacy bool
}

func (b *configBuilder) Build() (*ProgramConfig, error) {
	cfg := &ProgramConfig{
		ProgramName: b.programPath(),
		LanguageId:  b.block.Language(),
		Directory:   b.dir(),
	}

	if b.useInteractiveLegacy {
		cfg.Interactive = b.block.InteractiveLegacy()
	} else {
		cfg.Interactive = b.block.Interactive()
	}

	if isShell(cfg) {
		cfg.Mode = runnerv2.CommandMode_COMMAND_MODE_INLINE
		cfg.Source = &runnerv2.ProgramConfig_Commands{
			Commands: &runnerv2.ProgramConfig_CommandList{
				Items: b.block.Lines(),
			},
		}
	} else {
		cfg.Mode = runnerv2.CommandMode_COMMAND_MODE_FILE
		cfg.Source = &runnerv2.ProgramConfig_Script{
			Script: strings.Join(b.block.Lines(), "\n"),
		}
	}

	return cfg, nil
}

func (b *configBuilder) dir() string {
	var dirs []string

	doc := b.block.Document()
	fmtr, err := doc.FrontmatterWithError()
	if err == nil && fmtr != nil && fmtr.Cwd != "" {
		dirs = append(dirs, fmtr.Cwd)
	}

	if dir := b.block.Cwd(); dir != "" {
		dirs = append(dirs, dir)
	}

	if cwd, err := os.Getwd(); err == nil {
		dirs = append(dirs, cwd)
	}

	// TODO(adamb): figure out the first argument.
	return resolveDir("", dirs)
}

func (b *configBuilder) programPath() (programPath string) {
	language := b.block.Language()

	// If the language is a shell language, check frontmatter for shell overwrite.
	if IsShellLanguage(language) {
		doc := b.block.Document()
		fmtr, err := doc.FrontmatterWithError()
		if err == nil && fmtr != nil && fmtr.Shell != "" {
			programPath = fmtr.Shell
		}
	}

	// Interpreter can be always overwritten at the block level.
	if val := b.block.Interpreter(); val != "" {
		programPath = val
	}

	return
}

func resolveDir(parentDir string, candidates []string) string {
	for _, dir := range candidates {
		dir := filepath.FromSlash(dir)
		newDir := resolveDirUsingParentAndChild(parentDir, dir)
		if stat, err := os.Stat(newDir); err == nil && stat.IsDir() {
			parentDir = newDir
		}
	}

	return parentDir
}

// TODO(adamb): figure out if it's needed and for which cases.
func resolveDirUsingParentAndChild(parent, child string) string {
	if child == "" {
		return parent
	}

	if filepath.IsAbs(child) {
		return child
	}

	if parent != "" {
		return filepath.Join(parent, child)
	}

	return child
}

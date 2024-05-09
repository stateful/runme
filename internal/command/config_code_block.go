package command

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/stateful/runme/v3/internal/document"
	runnerv2alpha1 "github.com/stateful/runme/v3/internal/gen/proto/go/runme/runner/v2alpha1"
)

func NewConfigFromCodeBlock(block *document.CodeBlock) (*Config, error) {
	return (&configBuilder{block: block}).Build()
}

type configBuilder struct {
	block *document.CodeBlock
}

func (b *configBuilder) Build() (*Config, error) {
	cfg := &Config{
		ProgramName: b.programPath(),
		LanguageId:  b.block.Language(),
		Directory:   b.dir(),
		Interactive: b.block.Interactive(),
	}

	if isShellLanguage(filepath.Base(cfg.ProgramName)) {
		cfg.Mode = runnerv2alpha1.CommandMode_COMMAND_MODE_INLINE
		cfg.Source = &runnerv2alpha1.ProgramConfig_Commands{
			Commands: &runnerv2alpha1.ProgramConfig_CommandList{
				Items: b.block.Lines(),
			},
		}
	} else {
		cfg.Mode = runnerv2alpha1.CommandMode_COMMAND_MODE_FILE
		cfg.Source = &runnerv2alpha1.ProgramConfig_Script{
			Script: prepareScriptFromLines(cfg.ProgramName, b.block.Lines()),
		}
	}

	return cfg, nil
}

func (b *configBuilder) dir() string {
	var dirs []string

	fmtr, err := b.block.Document().Frontmatter()
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
	programPath = ""
	language := b.block.Language()

	// If the language is a shell language, check frontmatter for shell overwrite.
	if isShellLanguage(language) {
		fmtr, err := b.block.Document().Frontmatter()
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

func prepareScriptFromLines(programPath string, lines []string) string {
	var buf strings.Builder

	for _, cmd := range lines {
		_, _ = buf.WriteString(cmd)
		_, _ = buf.WriteRune('\n')
	}

	return buf.String()
}

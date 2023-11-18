package project

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-billy/v5/util"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	"github.com/stateful/runme/internal/document"
	"github.com/stateful/runme/internal/document/identity"
)

type LoadEventType uint8

const (
	LoadEventStartedWalk LoadEventType = iota + 1
	LoadEventFoundDir
	LoadEventFoundFile
	LoadEventFinishedWalk
	LoadEventStartedParsingDocument
	LoadEventFinishedParsingDocument
	LoadEventFoundTask
	LoadEventError
)

type LoadEvent struct {
	Type LoadEventType
	Data any
}

type ProjectOption func(*Project)

func WithRespectGitignore() ProjectOption {
	return func(p *Project) {
		p.respectGitignore = true
	}
}

func WithIgnoreFilePatterns(patterns ...string) ProjectOption {
	return func(p *Project) {
		p.ignoreFilePatterns = append(p.ignoreFilePatterns, patterns...)
	}
}

func WithFindRepoUpward() ProjectOption {
	return func(p *Project) {
		if p.plainOpenOptions == nil {
			p.plainOpenOptions = &git.PlainOpenOptions{}
		}

		p.plainOpenOptions.DetectDotGit = true
	}
}

func WithEnvRelFilenames(orderFilenames []string) ProjectOption {
	return func(p *Project) {
		p.envRelFilenames = orderFilenames
	}
}

func WithIdentityResolver(resolver *identity.IdentityResolver) ProjectOption {
	return func(p *Project) {
		p.identityResolver = resolver
	}
}

type Project struct {
	identityResolver *identity.IdentityResolver

	// filePath is used for file-based projects.
	filePath string

	// fs is used for dir-based projects.
	fs billy.Filesystem
	// ignoreFilePatterns is used for dir-based projects to
	// ignore certain file patterns.
	ignoreFilePatterns []string

	// Used when dir project is or is within a git repository.
	repo             *git.Repository
	plainOpenOptions *git.PlainOpenOptions
	respectGitignore bool

	// envRelFilenames contains an ordered list of file names,
	// relative to dir-based projects, from which envs
	// should be loaded.
	envRelFilenames []string
}

func NewDirProject(
	dir string,
	opts ...ProjectOption,
) (*Project, error) {
	p := &Project{}

	for _, opt := range opts {
		opt(p)
	}

	if _, err := os.Stat(dir); err != nil {
		return nil, errors.WithStack(err)
	}

	if p.plainOpenOptions != nil {
		var err error
		p.repo, err = git.PlainOpenWithOptions(
			dir,
			p.plainOpenOptions,
		)
		if err != nil {
			return nil, errors.WithStack(err)
		}
	}

	p.fs = osfs.New(dir)

	return p, nil
}

func NewFileProject(
	path string,
	opts ...ProjectOption,
) (*Project, error) {
	p := &Project{}

	for _, opt := range opts {
		opt(p)
	}

	if !filepath.IsAbs(path) {
		var err error
		path, err = filepath.Abs(path)
		if err != nil {
			return nil, errors.WithStack(err)
		}
	}

	if _, err := os.Stat(path); err != nil {
		return nil, errors.WithStack(err)
	}

	p.filePath = path

	return p, nil
}

func (p *Project) Root() string {
	if p.filePath != "" {
		return filepath.Dir(p.filePath)
	}

	if p.fs != nil {
		return p.fs.Root()
	}

	panic("invariant: Project was not initialized properly")
}

func (p *Project) EnvRelFilenames() []string {
	return p.envRelFilenames
}

func (p *Project) LoadEnvs() ([]string, error) {
	if p.fs == nil {
		return nil, nil
	}

	var envs []string

	for _, envFile := range p.envRelFilenames {
		bytes, err := util.ReadFile(p.fs, envFile)
		if err != nil {
			// TODO(adamb): log this error
			var pathError *os.PathError
			if !errors.As(err, &pathError) {
				return nil, errors.WithStack(err)
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
			envs = append(envs, k+"="+v)
		}
	}

	return envs, nil
}

func (p *Project) Load(
	ctx context.Context,
	eventc chan<- LoadEvent,
	onlyFiles bool,
) {
	defer close(eventc)

	switch {
	case p.repo != nil:
		// Git-based project.
		// TODO: confirm if the order of appending to ignorePatterns is important.
		ignorePatterns := []gitignore.Pattern{
			// Ignore .git by default.
			gitignore.ParsePattern(".git", nil),
			gitignore.ParsePattern(".git/*", nil),
		}

		if p.respectGitignore {
			patterns, err := gitignore.ReadPatterns(p.fs, nil)
			if err != nil {
				eventc <- LoadEvent{
					Type: LoadEventError,
					Data: errors.WithStack(err),
				}
			}
			ignorePatterns = append(ignorePatterns, patterns...)
		}

		for _, p := range p.ignoreFilePatterns {
			ignorePatterns = append(ignorePatterns, gitignore.ParsePattern(p, nil))
		}

		p.loadFromDirectory(ctx, eventc, ignorePatterns, onlyFiles)
	case p.fs != nil:
		// Dir-based project.
		ignorePatterns := make([]gitignore.Pattern, 0, len(p.ignoreFilePatterns))

		// It's allowed for a dir-based project to read
		// .gitignore and interpret it.
		if p.respectGitignore {
			patterns, err := gitignore.ReadPatterns(p.fs, nil)
			if err != nil {
				eventc <- LoadEvent{
					Type: LoadEventError,
					Data: errors.WithStack(err),
				}
			}
			ignorePatterns = append(ignorePatterns, patterns...)
		}

		for _, p := range p.ignoreFilePatterns {
			ignorePatterns = append(ignorePatterns, gitignore.ParsePattern(p, nil))
		}

		p.loadFromDirectory(ctx, eventc, ignorePatterns, onlyFiles)
	case p.filePath != "":
		p.loadFromFile(ctx, eventc, p.filePath, onlyFiles)
	default:
		eventc <- LoadEvent{
			Type: LoadEventError,
			Data: errors.New("invariant violation: Project struct initialized incorrectly"),
		}
	}
}

func (p *Project) loadFromDirectory(
	ctx context.Context,
	eventc chan<- LoadEvent,
	ignorePatterns []gitignore.Pattern,
	onlyFiles bool,
) {
	filesToSearchBlocks := make([]string, 0)
	onFileFound := func(path string) {
		if !onlyFiles {
			filesToSearchBlocks = append(filesToSearchBlocks, path)
		}
	}

	ignoreMatcher := gitignore.NewMatcher(ignorePatterns)

	eventc <- LoadEvent{Type: LoadEventStartedWalk}

	err := util.Walk(p.fs, ".", func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		ignored := ignoreMatcher.Match(
			[]string{path},
			info.IsDir(),
		)
		if !ignored {
			absPath := p.fs.Join(p.fs.Root(), path)

			if info.IsDir() {
				eventc <- LoadEvent{
					Type: LoadEventFoundDir,
					Data: absPath,
				}
			} else if isMarkdown(path) {
				eventc <- LoadEvent{
					Type: LoadEventFoundFile,
					Data: absPath,
				}

				onFileFound(absPath)
			}
		} else if info.IsDir() {
			return filepath.SkipDir
		}

		return nil
	})
	if err != nil {
		eventc <- LoadEvent{
			Type: LoadEventError,
			Data: err,
		}
	}

	eventc <- LoadEvent{
		Type: LoadEventFinishedWalk,
	}

	if len(filesToSearchBlocks) == 0 {
		return
	}

	for _, file := range filesToSearchBlocks {
		extractTasksFromFile(ctx, eventc, file)
	}
}

func (p *Project) loadFromFile(
	ctx context.Context,
	eventc chan<- LoadEvent,
	path string,
	onlyFiles bool,
) {
	eventc <- LoadEvent{Type: LoadEventStartedWalk}

	eventc <- LoadEvent{
		Type: LoadEventFoundFile,
		Data: path,
	}

	eventc <- LoadEvent{
		Type: LoadEventFinishedWalk,
	}

	if onlyFiles {
		return
	}

	extractTasksFromFile(ctx, eventc, path)
}

func extractTasksFromFile(
	ctx context.Context,
	eventc chan<- LoadEvent,
	path string,
) {
	eventc <- LoadEvent{
		Type: LoadEventStartedParsingDocument,
		Data: path,
	}

	codeBlocks, err := getCodeBlocksFromFile(path)

	eventc <- LoadEvent{
		Type: LoadEventFinishedParsingDocument,
		Data: path,
	}

	if err != nil {
		eventc <- LoadEvent{
			Type: LoadEventError,
			Data: err,
		}
	}

	for _, b := range codeBlocks {
		eventc <- LoadEvent{
			Type: LoadEventFoundTask,
			Data: CodeBlock{
				Filename:  path,
				CodeBlock: b,
			},
		}
	}
}

func getCodeBlocksFromFile(path string) (document.CodeBlocks, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return getCodeBlocks(data)
}

func getCodeBlocks(data []byte) (document.CodeBlocks, error) {
	identityResolver := identity.NewResolver(identity.DefaultLifecycleIdentity)
	d := document.New(data, identityResolver)
	node, err := d.Root()
	if err != nil {
		return nil, err
	}
	return document.CollectCodeBlocks(node), nil
}

func isMarkdown(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	return ext == ".md" || ext == ".mdx" || ext == ".mdi" || ext == ".mdr" || ext == ".run" || ext == ".runme"
}

package project

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-billy/v5/util"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"github.com/pkg/errors"
	"github.com/stateful/runme/internal/document"
	"github.com/stateful/runme/internal/document/identity"
	"go.uber.org/multierr"
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

type (
	LoadEventStartedWalkData  struct{}
	LoadEventFinishedWalkData struct{}

	LoadEventFoundDirData struct {
		Path string
	}

	LoadEventFoundFileData struct {
		Path string
	}

	LoadEventStartedParsingDocumentData struct {
		Path string
	}

	LoadEventFinishedParsingDocumentData struct {
		Path string
	}

	LoadEventFoundTaskData struct {
		DocumentPath    string
		ID              string
		Name            string
		IsNameGenerated bool
	}

	LoadEventErrorData struct {
		Err error
	}
)

type LoadEvent struct {
	Type LoadEventType
	Data any
}

// TODO(adamb): add more robust implementation.
//
// Consider switching away from reflection
// as this method is used in hot code path.
func (e LoadEvent) extractDataValue(val any) {
	reflect.ValueOf(val).Elem().Set(reflect.ValueOf(e.Data))
}

func ExtractDataFromLoadEvent[T any](event LoadEvent) T {
	var data T
	event.extractDataValue(&data)
	return data
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

	// For compatibility, but currently no option is
	// valid for file projects,
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

func (p *Project) Load(
	ctx context.Context,
	eventc chan<- LoadEvent,
	onlyFiles bool,
) {
	defer close(eventc)

	switch {
	case p.repo != nil:
		ignorePatterns, err := p.getAllIgnorePatterns()
		if err != nil {
			p.send(ctx, eventc, LoadEvent{
				Type: LoadEventError,
				Data: LoadEventErrorData{Err: err},
			})
		}

		p.loadFromDirectory(ctx, eventc, ignorePatterns, onlyFiles)
	case p.fs != nil:
		ignorePatterns, err := p.getAllIgnorePatterns()
		if err != nil {
			p.send(ctx, eventc, LoadEvent{
				Type: LoadEventError,
				Data: LoadEventErrorData{Err: err},
			})
		}

		p.loadFromDirectory(ctx, eventc, ignorePatterns, onlyFiles)
	case p.filePath != "":
		p.loadFromFile(ctx, eventc, p.filePath, onlyFiles)
	default:
		p.send(ctx, eventc, LoadEvent{
			Type: LoadEventError,
			Data: LoadEventErrorData{Err: errors.New("invariant violation: Project struct initialized incorrectly")},
		})
	}
}

func (p *Project) send(ctx context.Context, eventc chan<- LoadEvent, event LoadEvent) {
	select {
	case eventc <- event:
	case <-ctx.Done():
	}
}

func (p *Project) getAllIgnorePatterns() (_ []gitignore.Pattern, err error) {
	// TODO: confirm if the order of appending to ignorePatterns is important.
	ignorePatterns := []gitignore.Pattern{
		// Ignore .git by default.
		gitignore.ParsePattern(".git", nil),
	}

	if p.respectGitignore {
		patterns, readErr := gitignore.ReadPatterns(p.fs, nil)
		if readErr != nil {
			err = multierr.Append(err, errors.WithStack(readErr))
		} else {
			ignorePatterns = append(ignorePatterns, patterns...)
		}
	}

	for _, p := range p.ignoreFilePatterns {
		ignorePatterns = append(ignorePatterns, gitignore.ParsePattern(p, nil))
	}

	return ignorePatterns, err
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

	p.send(ctx, eventc, LoadEvent{Type: LoadEventStartedWalk})

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
				p.send(ctx, eventc, LoadEvent{
					Type: LoadEventFoundDir,
					Data: LoadEventFoundDirData{Path: absPath},
				})
			} else if isMarkdown(path) {
				p.send(ctx, eventc, LoadEvent{
					Type: LoadEventFoundFile,
					Data: LoadEventFoundFileData{Path: absPath},
				})

				onFileFound(absPath)
			}
		} else if info.IsDir() {
			return filepath.SkipDir
		}

		return nil
	})
	if err != nil {
		p.send(ctx, eventc, LoadEvent{
			Type: LoadEventError,
			Data: LoadEventErrorData{Err: err},
		})
	}

	p.send(ctx, eventc, LoadEvent{
		Type: LoadEventFinishedWalk,
	})

	if len(filesToSearchBlocks) == 0 {
		return
	}

	for _, file := range filesToSearchBlocks {
		p.extractTasksFromFile(ctx, eventc, file)
	}
}

func (p *Project) loadFromFile(
	ctx context.Context,
	eventc chan<- LoadEvent,
	path string,
	onlyFiles bool,
) {
	p.send(ctx, eventc, LoadEvent{Type: LoadEventStartedWalk})
	p.send(ctx, eventc, LoadEvent{
		Type: LoadEventFoundFile,
		Data: LoadEventFoundFileData{Path: path},
	})
	p.send(ctx, eventc, LoadEvent{
		Type: LoadEventFinishedWalk,
	})

	if onlyFiles {
		return
	}

	p.extractTasksFromFile(ctx, eventc, path)
}

func (p *Project) extractTasksFromFile(
	ctx context.Context,
	eventc chan<- LoadEvent,
	path string,
) {
	p.send(ctx, eventc, LoadEvent{
		Type: LoadEventStartedParsingDocument,
		Data: LoadEventStartedParsingDocumentData{Path: path},
	})

	codeBlocks, err := getCodeBlocksFromFile(path)

	p.send(ctx, eventc, LoadEvent{
		Type: LoadEventFinishedParsingDocument,
		Data: LoadEventFinishedParsingDocumentData{Path: path},
	})

	if err != nil {
		p.send(ctx, eventc, LoadEvent{
			Type: LoadEventError,
			Data: LoadEventErrorData{Err: err},
		})
	}

	for _, b := range codeBlocks {
		p.send(ctx, eventc, LoadEvent{
			Type: LoadEventFoundTask,
			Data: LoadEventFoundTaskData{
				DocumentPath:    path,
				ID:              b.ID(),
				Name:            b.Name(),
				IsNameGenerated: b.IsUnnamed(),
			},
		})
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

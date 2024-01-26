package project

import (
	"context"
	"fmt"
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
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	"go.uber.org/zap"

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
		Task Task
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

var DefaultProjectOptions = [...]ProjectOption{
	WithFindRepoUpward(),
	WithRespectGitignore(true),
	WithEnvFilesReadOrder([]string{".env"}),
	WithIgnoreFilePatterns("node_modules", ".venv", "vendor"),
}

type ProjectOption func(*Project)

func WithRespectGitignore(value bool) ProjectOption {
	return func(p *Project) {
		p.respectGitignore = value
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

func WithEnvFilesReadOrder(order []string) ProjectOption {
	return func(p *Project) {
		if len(order) == 0 {
			return
		}
		p.envFilesReadOrder = order
	}
}

func WithLogger(logger *zap.Logger) ProjectOption {
	return func(p *Project) {
		p.logger = logger
	}
}

type Project struct {
	// filePath is used for file-based projects.
	filePath string

	// fs is used for dir-based projects.
	fs billy.Filesystem
	// ignoreFilePatterns is used for dir-based projects to
	// ignore certain file patterns.
	ignoreFilePatterns []string

	// Used when dir project is or is within a git repository.
	// `repo`, if not nil, only indicates that the directory
	// contains a valid .git directory. It's not used for anything.
	repo             *git.Repository
	plainOpenOptions *git.PlainOpenOptions
	respectGitignore bool

	// envFilesReadOrder is a list of paths to .env files
	// to read from.
	envFilesReadOrder []string

	logger *zap.Logger
}

// normalizeAndValidatePath makes sure that the path is absolute and
// checks if the path exists.
func normalizeAndValidatePath(path string) (string, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return "", errors.WithStack(err)
	}

	if _, err := os.Stat(path); err != nil {
		// Handle ErrNotExist to provide more user-friendly error message.
		if errors.Is(err, os.ErrNotExist) {
			return "", errors.Wrapf(os.ErrNotExist, "failed to open file-based project %q", path)
		}
		return "", errors.WithStack(err)
	}

	return path, nil
}

func NewDirProject(
	dir string,
	opts ...ProjectOption,
) (*Project, error) {
	p := &Project{}

	for _, opt := range opts {
		opt(p)
	}

	var err error

	dir, err = normalizeAndValidatePath(dir)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open dir-based project %q", dir)
	}

	p.fs = osfs.New(dir)

	openOptions := p.plainOpenOptions

	if openOptions == nil {
		openOptions = &git.PlainOpenOptions{}
	}

	p.repo, err = git.PlainOpenWithOptions(
		dir,
		openOptions,
	)
	if err != nil && !errors.Is(err, git.ErrRepositoryNotExists) {
		return nil, errors.Wrapf(err, "failed to open dir-based project %q", dir)
	}

	if p.repo != nil {
		wt, err := p.repo.Worktree()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to open dir-based project %q", dir)
		}
		p.fs = wt.Filesystem
	}

	if p.logger == nil {
		p.logger = zap.NewNop()
	}

	return p, nil
}

func NewFileProject(
	path string,
	opts ...ProjectOption,
) (*Project, error) {
	p := &Project{}

	// For compatibility; many options are not used for file-based projects.
	for _, opt := range opts {
		opt(p)
	}

	var err error

	path, err = normalizeAndValidatePath(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open file-based project %q", path)
	}

	p.filePath = path

	if p.logger == nil {
		p.logger = zap.NewNop()
	}

	return p, nil
}

func (p *Project) String() string {
	switch {
	case p.filePath != "":
		return fmt.Sprintf("File-based project with path %q", p.filePath)
	case p.repo != nil:
		return fmt.Sprintf("Git-based project in directory %q", p.fs.Root())
	case p.fs != nil:
		return fmt.Sprintf("Directory-based project in %q", p.fs.Root())
	default:
		return "Invalid project"
	}
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

func (p *Project) EnvFilesReadOrder() []string {
	return p.envFilesReadOrder
}

type LoadOptions struct {
	OnlyFiles bool
}

func (p *Project) Load(
	ctx context.Context,
	eventc chan<- LoadEvent,
	onlyFiles bool,
) {
	p.load(ctx, eventc, LoadOptions{OnlyFiles: onlyFiles})
}

func (p *Project) LoadWithOptions(
	ctx context.Context,
	eventc chan<- LoadEvent,
	options LoadOptions,
) {
	p.load(ctx, eventc, options)
}

func (p *Project) load(
	ctx context.Context,
	eventc chan<- LoadEvent,
	options LoadOptions,
) {
	defer close(eventc)

	switch {
	case p.repo != nil:
		// The logic is identical to a dir-based project because
		// we adjust the root to the repo's in the ctor
		fallthrough
	case p.fs != nil:
		p.loadFromDirectory(ctx, eventc, options)
	case p.filePath != "":
		p.loadFromFile(ctx, eventc, p.filePath, options)
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

func (p *Project) getAllIgnorePatterns() []gitignore.Pattern {
	// TODO: confirm if the order of appending to ignorePatterns is important.
	ignorePatterns := []gitignore.Pattern{
		// Ignore .git by default.
		gitignore.ParsePattern(".git", nil),
	}

	if p.respectGitignore {
		sysPatterns, err := gitignore.LoadSystemPatterns(p.fs)
		if err != nil {
			p.logger.Info("failed to load system ignore patterns", zap.Error(err))
		}
		ignorePatterns = append(ignorePatterns, sysPatterns...)

		globPatterns, err := gitignore.LoadGlobalPatterns(p.fs)
		if err != nil {
			p.logger.Info("failed to load global ignore patterns", zap.Error(err))
		}
		ignorePatterns = append(ignorePatterns, globPatterns...)

		// TODO(adamb): this is a slow operation if there are many directories.
		// Profile this function and figure out a way to optimize it.
		patterns, err := gitignore.ReadPatterns(p.fs, nil)
		if err != nil {
			p.logger.Info("failed to load local ignore patterns", zap.Error(err))
		}
		ignorePatterns = append(ignorePatterns, patterns...)
	}

	for _, p := range p.ignoreFilePatterns {
		ignorePatterns = append(ignorePatterns, gitignore.ParsePattern(p, nil))
	}

	return ignorePatterns
}

func (p *Project) loadFromDirectory(
	ctx context.Context,
	eventc chan<- LoadEvent,
	options LoadOptions,
) {
	filesToSearchBlocks := make([]string, 0)
	onFileFound := func(path string) {
		if !options.OnlyFiles {
			filesToSearchBlocks = append(filesToSearchBlocks, path)
		}
	}

	ignorePatterns := p.getAllIgnorePatterns()
	ignoreMatcher := gitignore.NewMatcher(ignorePatterns)

	p.send(ctx, eventc, LoadEvent{Type: LoadEventStartedWalk})

	err := util.Walk(p.fs, ".", func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		ignored := ignoreMatcher.Match(
			strings.Split(path, string(filepath.Separator)),
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
	options LoadOptions,
) {
	p.send(ctx, eventc, LoadEvent{Type: LoadEventStartedWalk})
	p.send(ctx, eventc, LoadEvent{
		Type: LoadEventFoundFile,
		Data: LoadEventFoundFileData{Path: path},
	})
	p.send(ctx, eventc, LoadEvent{
		Type: LoadEventFinishedWalk,
	})

	if options.OnlyFiles {
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
				Task: Task{
					CodeBlock:    b,
					DocumentPath: path,
				},
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

	if f, err := d.Frontmatter(); err == nil && f != nil && f.Runme.Session.ID != "" {
		return nil, nil
	}

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

func (p *Project) LoadEnv() ([]string, error) {
	envs, err := p.LoadEnvAsMap()
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, len(envs))

	for k, v := range envs {
		result = append(result, k+"="+v)
	}

	return result, nil
}

func (p *Project) LoadEnvAsMap() (map[string]string, error) {
	// For file-based projects, there are no envs to read.
	if p.fs == nil {
		return nil, nil
	}

	envs := make(map[string]string)

	for _, envFile := range p.envFilesReadOrder {
		bytes, err := util.ReadFile(p.fs, envFile)

		var pathError *os.PathError
		if err != nil {
			if !errors.As(err, &pathError) {
				return nil, errors.Wrapf(err, "failed to read .env file %q", envFile)
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

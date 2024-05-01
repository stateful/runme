package config

import (
	"io/fs"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

var ErrRootConfigNotFound = errors.New("root configuration file not found")

// Loader allows to load configuration files from a file system.
type Loader struct {
	// configRootPath is a root path for the configuration file.
	// Typically, it's a project root path, which currently defaults to
	// the current working directory.
	configRootPath fs.FS

	// configName is a name of the configuration file.
	configName string

	// configType is a type of the configuration file.
	// Together with configName it forms a configFile.
	configType string

	// projectRootPath is a path to the project root directory.
	// If not empty, it is used to find nested configuration files,
	// for example using [ChainConfigs], instead of configRootPath.
	projectRootPath fs.FS

	logger *zap.Logger
}

type LoaderOption func(*Loader)

func WithLogger(logger *zap.Logger) LoaderOption {
	return func(l *Loader) {
		l.logger = logger
	}
}

func WithProjectRootPath(projectRootPath fs.FS) LoaderOption {
	return func(l *Loader) {
		l.projectRootPath = projectRootPath
	}
}

func NewLoader(configName, configType string, configRootPath fs.FS, opts ...LoaderOption) *Loader {
	if configName == "" {
		panic("config name is not set")
	}

	l := &Loader{
		configRootPath: configRootPath,
		configName:     configName,
		configType:     configType,
	}

	for _, opt := range opts {
		opt(l)
	}

	if l.logger == nil {
		l.logger = zap.NewNop()
	}

	return l
}

func (l *Loader) configFullName() string {
	if l.configType == "" {
		return l.configName
	}
	return l.configName + "." + l.configType
}

func (l *Loader) SetConfigRootPath(configRootPath fs.FS) {
	l.configRootPath = configRootPath
}

func (l *Loader) FindConfigChain(path string) ([][]byte, error) {
	paths, err := l.findConfigFilesOnPath(path)
	if err != nil {
		return nil, err
	}
	return l.readFiles(paths...)
}

func (l *Loader) RootConfig() ([]byte, error) {
	data, err := fs.ReadFile(l.configRootPath, l.configFullName())
	if err != nil {
		return nil, ErrRootConfigNotFound
	}
	return data, nil
}

func (l *Loader) findConfigFilesOnPath(name string) (result []string, _ error) {
	name, err := l.parsePath(name)
	if err != nil {
		return nil, err
	}
	l.logger.Debug("finding config files on path", zap.String("name", name))

	configFullName := l.configFullName()

	// Find the root configuration file and add it to the result if exists.
	// It is always searched in the config root directory.
	_, err = fs.Stat(l.configRootPath, configFullName)
	if err == nil {
		result = append(result, configFullName)
	} else if !errors.Is(err, fs.ErrNotExist) {
		l.logger.Debug("root configuration file not found", zap.Error(err))
		return nil, err
	}

	// Detect the file system to use for nested configuration files.
	fsys := l.configRootPath
	if l.projectRootPath != nil {
		fsys = l.projectRootPath
	}

	// Split the path and iterate over the fragments to find nested configuration files.
	fragments := strings.Split(name, string(filepath.Separator))
	if len(fragments) > 0 && fragments[0] == "." {
		fragments = fragments[1:]
	}
	l.logger.Debug("path fragments", zap.Strings("fragments", fragments))

	curDir := ""
	for _, fragment := range fragments {
		// Use [path.Join] instead of [filepath.Join] to support Windows paths.
		// It works well with [fs.FS].
		curDir = path.Join(curDir, fragment)

		configPath := path.Join(curDir, configFullName)
		l.logger.Debug("checking nested configuration file", zap.String("path", configPath))
		_, err := fs.Stat(fsys, configPath)
		if err == nil {
			result = append(result, configPath)
		} else if !errors.Is(err, fs.ErrNotExist) {
			l.logger.Debug("nested configuration file not found", zap.String("path", configPath), zap.Error(err))
			return nil, err
		}
	}

	l.logger.Debug("found config files on path", zap.String("name", name), zap.Strings("files", result))

	return result, nil
}

func (l *Loader) parsePath(name string) (string, error) {
	if name == "" {
		name = "."
	}

	fsys := l.configRootPath
	if l.projectRootPath != nil {
		fsys = l.projectRootPath
	}

	info, err := fs.Stat(fsys, name)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get the path info for %q", name)
	}

	if info.IsDir() {
		return filepath.Clean(name), nil
	}
	return filepath.Dir(name), nil
}

func (l *Loader) readFiles(paths ...string) (result [][]byte, _ error) {
	for _, path := range paths {
		data, err := fs.ReadFile(l.configRootPath, path)
		if err != nil {
			return nil, err
		}
		result = append(result, data)
	}
	return result, nil
}

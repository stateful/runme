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
	// configRootPaths is a list of root paths for the configuration file.
	// The first found file is used as the root configuration file.
	configRootPaths []fs.FS

	// configName is a name of the configuration file.
	configName string

	// configType is a type of the configuration file.
	// Together with configName it forms a configFile.
	configType string

	// projectRootPath is a path to the project root directory.
	// If not empty, it is used to find nested configuration files,
	// for example using [ChainConfigs].
	projectRootPath fs.FS

	logger *zap.Logger
}

type LoaderOption func(*Loader)

func WithLogger(logger *zap.Logger) LoaderOption {
	return func(l *Loader) {
		l.logger = logger
	}
}

func WithAdditionalConfigPath(path fs.FS) LoaderOption {
	return func(l *Loader) {
		l.configRootPaths = append(l.configRootPaths, path)
	}
}

func NewLoader(configName, configType string, projectPath fs.FS, opts ...LoaderOption) *Loader {
	if configName == "" {
		panic("config name is not set")
	}

	l := &Loader{
		configName:      configName,
		configType:      configType,
		projectRootPath: projectPath,
	}

	for _, opt := range opts {
		opt(l)
	}

	if l.logger == nil {
		l.logger = zap.NewNop()
	}

	return l
}

func (l *Loader) FindConfigChain(path string) ([][]byte, error) {
	return l.findConfigChain(path)
}

func (l *Loader) RootConfig() ([]byte, error) {
	name := l.configFullName()

	for _, fsys := range l.configRootPaths {
		_, err := fs.Stat(fsys, name)
		if err == nil {
			data, err := fs.ReadFile(fsys, name)
			if err != nil {
				return nil, errors.WithStack(err)
			}
			return data, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, errors.WithStack(err)
		}
	}

	data, err := fs.ReadFile(l.projectRootPath, name)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrRootConfigNotFound
		}
		return nil, errors.WithStack(err)
	}
	return data, nil
}

func (l *Loader) SetConfigRootPaths(configRootPaths ...fs.FS) {
	l.configRootPaths = configRootPaths
}

func (l *Loader) configFullName() string {
	if l.configType == "" {
		return l.configName
	}
	return l.configName + "." + l.configType
}

func (l *Loader) findConfigChain(name string) (result [][]byte, _ error) {
	name, err := l.cleanPath(name)
	if err != nil {
		return nil, err
	}
	l.logger.Debug("finding config files on path", zap.String("name", name))

	// Config chanin starts with the root configuration file.
	rootConfig, err := l.RootConfig()
	if err != nil {
		return nil, err
	}
	result = append(result, rootConfig)

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

		path := path.Join(curDir, l.configFullName())
		l.logger.Debug("checking nested configuration file", zap.String("path", path))

		data, err := fs.ReadFile(l.projectRootPath, path)
		if err == nil {
			result = append(result, data)
		} else if !errors.Is(err, fs.ErrNotExist) {
			l.logger.Debug("error while reading nested configuration file", zap.String("path", path), zap.Error(err))
			return nil, err
		}
	}

	return result, nil
}

func (l *Loader) cleanPath(name string) (string, error) {
	if name == "" {
		name = "."
	}
	info, err := fs.Stat(l.projectRootPath, name)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get the path info for %q", name)
	}
	if info.IsDir() {
		return filepath.Clean(name), nil
	}
	return filepath.Dir(name), nil
}

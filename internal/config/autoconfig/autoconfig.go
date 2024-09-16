// autoconfig provides a way to create instances of objects based on
// the configuration in runme.yaml.
//
// For example, to instantiate [project.Project], you can write:
//
//	autoconfig.Invoke(func(p *project.Project) error {
//	    ...
//	})
//
// Treat it as a dependency injection mechanism.
package autoconfig

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"go.uber.org/dig"
	"go.uber.org/zap"

	"github.com/stateful/runme/v3/internal/command"
	"github.com/stateful/runme/v3/internal/config"
	"github.com/stateful/runme/v3/internal/dockerexec"
	"github.com/stateful/runme/v3/pkg/project"
)

var (
	container    = dig.New()
	commandScope = container.Scope("command")
	serverScope  = container.Scope("server")
)

func DecorateRoot(decorator interface{}, opts ...dig.DecorateOption) error {
	return container.Decorate(decorator, opts...)
}

// InvokeForCommand is used to invoke the function with the given dependencies.
// The package will automatically figure out how to instantiate them
// using the available configuration.
//
// Use it only for commands because it supports only singletons
// created during the program initialization.
func InvokeForCommand(function interface{}, opts ...dig.InvokeOption) error {
	err := commandScope.Invoke(function, opts...)
	return dig.RootCause(err)
}

// InvokeForServer is similar to InvokeForCommand, but it does not provide
// all the dependencies, in particular, it does not provide dependencies
// that differ per request.
func InvokeForServer(function interface{}, opts ...dig.InvokeOption) error {
	err := serverScope.Invoke(function, opts...)
	return dig.RootCause(err)
}

func mustProvide(err error) {
	if err != nil {
		panic("failed to provide: " + err.Error())
	}
}

func init() {
	mustProvide(container.Provide(getCommandFactory))
	mustProvide(container.Provide(getConfigLoader))
	mustProvide(container.Provide(getDocker))
	mustProvide(container.Provide(getLogger))
	mustProvide(container.Provide(getProject))
	mustProvide(container.Provide(getProjectFilters))
	mustProvide(container.Provide(getRootConfig))
	mustProvide(container.Provide(getUserConfigDir))
}

func getCommandFactory(docker *dockerexec.Docker, logger *zap.Logger, proj *project.Project) command.Factory {
	return command.NewFactory(
		command.WithDocker(docker),
		command.WithLogger(logger),
		command.WithProject(proj),
	)
}

func getConfigLoader() (*config.Loader, error) {
	// TODO(adamb): change from "./experimental" to "." when the feature is stable and
	// delete the project root path.
	return config.NewLoader("runme", "yaml", os.DirFS("./experimental"), config.WithProjectRootPath(os.DirFS("."))), nil
}

func getDocker(c *config.Config, logger *zap.Logger) (*dockerexec.Docker, error) {
	if !c.RuntimeDockerEnabled {
		return nil, nil
	}

	return dockerexec.New(&dockerexec.Options{
		BuildContext: c.RuntimeDockerBuildContext,
		Dockerfile:   c.RuntimeDockerBuildDockerfile,
		Image:        c.RuntimeDockerImage,
		Logger:       logger,
	})
}

func getLogger(c *config.Config) (*zap.Logger, error) {
	if c == nil || !c.LogEnabled {
		return zap.NewNop(), nil
	}

	zapConfig := zap.Config{
		Level:       zap.NewAtomicLevelAt(zap.InfoLevel),
		Development: false,
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
		Encoding:         "json",
		EncoderConfig:    zap.NewProductionEncoderConfig(),
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}

	if c.LogVerbose {
		zapConfig.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
		zapConfig.Development = true
		zapConfig.Encoding = "console"
		zapConfig.EncoderConfig = zap.NewDevelopmentEncoderConfig()
	}

	if c.LogPath != "" {
		zapConfig.OutputPaths = []string{c.LogPath}
		zapConfig.ErrorOutputPaths = []string{c.LogPath}
	}

	l, err := zapConfig.Build()
	return l, errors.WithStack(err)
}

func getProject(c *config.Config, logger *zap.Logger) (*project.Project, error) {
	opts := []project.ProjectOption{
		project.WithLogger(logger),
	}

	if c.ProjectFilename != "" {
		return project.NewFileProject(c.ProjectFilename, opts...)
	}

	projDir := c.ProjectRoot
	// If no project directory is specified, use the current directory.
	if projDir == "" {
		projDir = "."
	}

	opts = append(
		opts,
		project.WithIgnoreFilePatterns(c.ProjectIgnorePaths...),
		project.WithRespectGitignore(!c.ProjectDisableGitignore),
		project.WithEnvFilesReadOrder(c.ProjectEnvSources),
	)

	if c.ProjectFindRepoUpward {
		opts = append(opts, project.WithFindRepoUpward())
	}

	return project.NewDirProject(projDir, opts...)
}

func getProjectFilters(c *config.Config) ([]project.Filter, error) {
	var filters []project.Filter

	for _, filter := range c.ProjectFilters {
		filter := filter

		switch filter.Type {
		case config.FilterTypeBlock:
			filters = append(filters, project.Filter(func(t project.Task) (bool, error) {
				env := config.FilterBlockEnv{
					Background: t.CodeBlock.Background(),
					// TODO(adamb): implement this in the code block.
					// CloseTerminalOnSuccess: t.CodeBlock.CloseTerminalOnSuccess(),
					Cwd:               t.CodeBlock.Cwd(),
					ExcludeFromRunAll: t.CodeBlock.ExcludeFromRunAll(),
					Interactive:       t.CodeBlock.InteractiveLegacy(),
					IsNamed:           !t.CodeBlock.IsUnnamed(),
					Language:          t.CodeBlock.Language(),
					Name:              t.CodeBlock.Name(),
					PromptEnv:         t.CodeBlock.PromptEnv(),
					Tags:              t.CodeBlock.Tags(),
				}
				return filter.Evaluate(env)
			}))
		case config.FilterTypeDocument:
			filters = append(filters, project.Filter(func(t project.Task) (bool, error) {
				doc := t.CodeBlock.Document()
				fmtr, err := doc.FrontmatterWithError()
				if err != nil {
					return false, err
				}
				if fmtr == nil {
					return false, nil
				}

				env := config.FilterDocumentEnv{
					Shell: fmtr.Shell,
					Cwd:   fmtr.Cwd,
				}
				return filter.Evaluate(env)
			}))
		default:
			return nil, errors.Errorf("unknown filter type: %s", filter.Type)
		}
	}

	return filters, nil
}

func getRootConfig(cfgLoader *config.Loader, userCfgDir UserConfigDir) (*config.Config, error) {
	var cfg *config.Config

	content, err := cfgLoader.RootConfig()
	switch err {
	case nil:
		if cfg, err = config.ParseYAML(content); err != nil {
			return nil, err
		}
	case config.ErrRootConfigNotFound:
		cfg = config.Defaults()
	default:
		return nil, errors.WithMessage(err, "failed to load root configuration")
	}

	if cfg.ServerTLSEnabled {
		if cfg.ServerTLSCertFile == "" {
			cfg.ServerTLSCertFile = filepath.Join(string(userCfgDir), "runme", "tls", "cert.pem")
		}
		if cfg.ServerTLSKeyFile == "" {
			cfg.ServerTLSKeyFile = filepath.Join(string(userCfgDir), "runme", "tls", "key.pem")
		}
	}

	return cfg, nil
}

type UserConfigDir string

func getUserConfigDir() (UserConfigDir, error) {
	dir, err := os.UserConfigDir()
	return UserConfigDir(dir), errors.WithStack(err)
}

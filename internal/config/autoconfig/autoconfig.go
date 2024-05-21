// autoconfig provides a way to create various instances from the [config.Config] like
// [project.Project], [command.Session], [zap.Logger].
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
	"github.com/stateful/runme/v3/internal/project"
)

var container = dig.New()

// Invoke is used to invoke the function with the given dependencies.
// The package will automatically figure out how to instantiate them
// using the available configuration.
func Invoke(function interface{}, opts ...dig.InvokeOption) error {
	err := container.Invoke(function, opts...)
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
	mustProvide(container.Provide(getLogger))
	mustProvide(container.Provide(getProject))
	mustProvide(container.Provide(getProjectFilters))
	mustProvide(container.Provide(getRootConfig))
	mustProvide(container.Provide(getSession))
	mustProvide(container.Provide(getRuntime))
	mustProvide(container.Provide(getUserConfigDir))
}

func getCommandFactory(cfg *config.Config, runtime command.Runtime, logger *zap.Logger) command.Factory {
	return command.NewFactory(cfg, runtime, logger)
}

func getConfigLoader() (*config.Loader, error) {
	// TODO(adamb): change from "./experimental" to "." when the feature is stable and
	// delete the project root path.
	return config.NewLoader("runme", "yaml", os.DirFS("./experimental"), config.WithProjectRootPath(os.DirFS("."))), nil
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

	if c.Filename != "" {
		return project.NewFileProject(c.Filename, opts...)
	}

	projDir := c.ProjectDir
	// If no project directory is specified, use the current directory.
	if projDir == "" {
		projDir = "."
	}

	opts = append(
		opts,
		project.WithIgnoreFilePatterns(c.IgnorePaths...),
		project.WithRespectGitignore(!c.DisableGitignore),
		project.WithEnvFilesReadOrder(c.EnvSourceFiles),
	)

	if c.FindRepoUpward {
		opts = append(opts, project.WithFindRepoUpward())
	}

	return project.NewDirProject(projDir, opts...)
}

func getProjectFilters(c *config.Config) ([]project.Filter, error) {
	var filters []project.Filter

	for _, filter := range c.Filters {
		filter := filter

		switch filter.Type {
		case config.FilterTypeBlock:
			filters = append(filters, project.Filter(func(t project.Task) (bool, error) {
				env := config.FilterBlockEnv{
					Background: t.CodeBlock.Background(),
					Categories: t.CodeBlock.Categories(),
					// TODO(adamb): implement this in the code block.
					// CloseTerminalOnSuccess: t.CodeBlock.CloseTerminalOnSuccess(),
					Cwd:               t.CodeBlock.Cwd(),
					ExcludeFromRunAll: t.CodeBlock.ExcludeFromRunAll(),
					Interactive:       t.CodeBlock.InteractiveLegacy(),
					IsNamed:           !t.CodeBlock.IsUnnamed(),
					Language:          t.CodeBlock.Language(),
					Name:              t.CodeBlock.Name(),
					PromptEnv:         t.CodeBlock.PromptEnv(),
				}
				return filter.Evaluate(env)
			}))
		case config.FilterTypeDocument:
			filters = append(filters, project.Filter(func(t project.Task) (bool, error) {
				fmtr, err := t.CodeBlock.Document().Frontmatter()
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
	content, err := cfgLoader.RootConfig()
	if err != nil {
		return nil, errors.WithMessage(err, "failed to load project configuration")
	}

	cfg, err := config.ParseYAML(content)
	if err != nil {
		return nil, err
	}

	if cfg.ServerTLSEnabled {
		if cfg.ServerTLSCertFile == "" {
			cfg.ServerTLSCertFile = filepath.Join(string(userCfgDir), "cert.pem")
		}
		if cfg.ServerTLSKeyFile == "" {
			cfg.ServerTLSKeyFile = filepath.Join(string(userCfgDir), "key.pem")
		}
	}

	return cfg, nil
}

func getRuntime(c *config.Config, logger *zap.Logger) (command.Runtime, error) {
	if c.DockerEnabled {
		docker, err := dockerexec.New(&dockerexec.Options{
			BuildContext: c.DockerBuildContext,
			Dockerfile:   c.DockerBuildDockerfile,
			Image:        c.DockerImage,
			Logger:       logger,
		})
		if err != nil {
			return nil, err
		}
		return command.NewDockerRuntime(docker), nil
	}
	return command.NewHostRuntime(), nil
}

func getSession(cfg *config.Config, proj *project.Project) (*command.Session, error) {
	sess := command.NewSession()

	if cfg.UseSystemEnv {
		if err := sess.SetEnv(os.Environ()...); err != nil {
			return nil, err
		}
	}

	if proj != nil {
		projEnv, err := proj.LoadEnv()
		if err != nil {
			return nil, err
		}

		if err := sess.SetEnv(projEnv...); err != nil {
			return nil, err
		}
	}

	return sess, nil
}

type UserConfigDir string

func getUserConfigDir() (UserConfigDir, error) {
	dir, err := os.UserConfigDir()
	return UserConfigDir(dir), errors.WithStack(err)
}

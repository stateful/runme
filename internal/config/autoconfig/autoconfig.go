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
//
// autoconfig relies on [viper.Viper] which has a set of limitations. The most important one
// is the fact that it does not support hierarchical configuration per folder. We might consider
// switchig from [viper.Viper] to something else in the future.
package autoconfig

import (
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"github.com/spf13/viper"
	"go.uber.org/dig"
	"go.uber.org/zap"

	"github.com/stateful/runme/v3/internal/command"
	"github.com/stateful/runme/v3/internal/config"
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
	// [viper.Viper] can be overridden by a decorator:
	//   container.Decorate(func(v *viper.Viper) *viper.Viper { return nil })
	mustProvide(container.Provide(getConfig))
	mustProvide(container.Provide(getLogger))
	mustProvide(container.Provide(getProject))
	mustProvide(container.Provide(getProjectFilters))
	mustProvide(container.Provide(getSession))
	mustProvide(container.Provide(getViper))

	if err := container.Invoke(func(viper *viper.Viper) {
		viper.SetConfigName("runme")
		viper.SetConfigType("yaml")

		viper.AddConfigPath("/etc/runme/")
		viper.AddConfigPath("$HOME/.runme/")
		// TODO(adamb): change to "." when ready.
		viper.AddConfigPath("experimental/")

		viper.SetEnvPrefix("RUNME")
		viper.AutomaticEnv()
	}); err != nil {
		panic("failed to setup configuration: " + err.Error())
	}
}

func getConfig(viper *viper.Viper) (*config.Config, error) {
	if err := viper.ReadInConfig(); err != nil {
		return nil, errors.WithStack(err)
	}

	// As viper does not offer writing config to a writer,
	// the workaround is to create a in-memory file system,
	// set it in viper, and write the config to it.
	// Finally, a deferred cleanup function is called
	// which brings back the OS file system.
	// Source: https://github.com/spf13/viper/issues/856
	memFS := afero.NewMemMapFs()

	viper.SetFs(memFS)
	defer viper.SetFs(afero.NewOsFs())

	if err := viper.WriteConfigAs("/config.yaml"); err != nil {
		return nil, errors.WithStack(err)
	}

	content, err := afero.ReadFile(memFS, "/config.yaml")
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return config.ParseYAML(content)
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
					Interactive:       t.CodeBlock.Interactive(),
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

func getViper() *viper.Viper { return viper.GetViper() }

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
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/stateful/runme/v3/internal/command"
	"github.com/stateful/runme/v3/internal/config"
	"github.com/stateful/runme/v3/internal/dockerexec"
	"github.com/stateful/runme/v3/internal/runnerv2client"
	runmetls "github.com/stateful/runme/v3/internal/tls"
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
	mustProvide(container.Provide(getClient))
	mustProvide(container.Provide(getClientFactory))
	mustProvide(container.Provide(getCommandFactory))
	mustProvide(container.Provide(getConfigLoader))
	mustProvide(container.Provide(getDocker))
	mustProvide(container.Provide(getLogger))
	mustProvide(container.Provide(getProject))
	mustProvide(container.Provide(getProjectFilters))
	mustProvide(container.Provide(getRootConfig))
	mustProvide(container.Provide(getUserConfigDir))
}

func getClient(cfg *config.Config, logger *zap.Logger) (*runnerv2client.Client, error) {
	if cfg.Server == nil {
		return nil, nil
	}

	var opts []grpc.DialOption

	if cfg.Server.Tls != nil && cfg.Server.Tls.Enabled {
		// It's ok to dereference TLS fields because they are checked in [getRootConfig].
		tlsConfig, err := runmetls.LoadClientConfig(*cfg.Server.Tls.CertFile, *cfg.Server.Tls.KeyFile)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		creds := credentials.NewTLS(tlsConfig)
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	return runnerv2client.New(
		cfg.Server.Address,
		logger,
		opts...,
	)
}

type ClientFactory func() (*runnerv2client.Client, error)

func getClientFactory(cfg *config.Config, logger *zap.Logger) ClientFactory {
	return func() (*runnerv2client.Client, error) {
		return getClient(cfg, logger)
	}
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
	return config.NewLoader(
		[]string{"runme.yaml", "runme." + os.Getenv("USER") + ".yaml"},
		os.DirFS("./experimental"),
		config.WithProjectRootPath(os.DirFS(".")),
	), nil
}

func getDocker(c *config.Config, logger *zap.Logger) (*dockerexec.Docker, error) {
	if c.Runtime == nil || c.Runtime.Docker == nil || !c.Runtime.Docker.Enabled {
		return nil, nil
	}

	options := &dockerexec.Options{
		Image:  c.Runtime.Docker.Image,
		Logger: logger,
	}

	if b := c.Runtime.Docker.Build; b != nil {
		options.BuildContext = c.Runtime.Docker.Build.Context
		options.Dockerfile = c.Runtime.Docker.Build.Dockerfile
	}

	return dockerexec.New(options)
}

func getLogger(c *config.Config) (*zap.Logger, error) {
	if c == nil || c.Log == nil || !c.Log.Enabled {
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

	if c.Log.Verbose {
		zapConfig.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
		zapConfig.Development = true
		zapConfig.Encoding = "console"
		zapConfig.EncoderConfig = zap.NewDevelopmentEncoderConfig()
	}

	if c.Log.Path != "" {
		zapConfig.OutputPaths = []string{c.Log.Path}
		zapConfig.ErrorOutputPaths = []string{c.Log.Path}
	}

	l, err := zapConfig.Build()
	return l, errors.WithStack(err)
}

func getProject(c *config.Config, logger *zap.Logger) (*project.Project, error) {
	opts := []project.ProjectOption{
		project.WithLogger(logger),
	}

	if c.Project.Filename != "" {
		return project.NewFileProject(c.Project.Filename, opts...)
	}

	projDir := c.Project.Root
	// If no project directory is specified, use the current directory.
	if projDir == "" {
		projDir = "."
	}

	opts = append(
		opts,
		project.WithIgnoreFilePatterns(c.Project.Ignore...),
		project.WithRespectGitignore(!c.Project.DisableGitignore),
		project.WithEnvFilesReadOrder(c.Project.Env.Sources),
	)

	if c.Project.FindRepoUpward {
		opts = append(opts, project.WithFindRepoUpward())
	}

	return project.NewDirProject(projDir, opts...)
}

func getProjectFilters(c *config.Config) ([]project.Filter, error) {
	var filters []project.Filter

	for _, filter := range c.Project.Filters {
		filter := config.Filter{
			Type:      string(filter.Type),
			Condition: filter.Condition,
			Extra:     filter.Extra,
		}

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

	items, err := cfgLoader.RootConfigs()
	switch err {
	case nil:
		if cfg, err = config.ParseYAML(items...); err != nil {
			return nil, err
		}
	case config.ErrRootConfigNotFound:
		cfg = config.Default()
	default:
		return nil, errors.WithMessage(err, "failed to load root configuration")
	}

	if cfg.Server != nil && cfg.Server.Tls != nil && cfg.Server.Tls.Enabled {
		tls := cfg.Server.Tls

		if tls.CertFile == nil {
			path := filepath.Join(string(userCfgDir), "runme", "tls", "cert.pem")
			tls.CertFile = &path
		}
		if tls.KeyFile == nil {
			path := filepath.Join(string(userCfgDir), "runme", "tls", "key.pem")
			tls.KeyFile = &path
		}
	}

	return cfg, nil
}

type UserConfigDir string

func getUserConfigDir() (UserConfigDir, error) {
	dir, err := os.UserConfigDir()
	return UserConfigDir(dir), errors.WithStack(err)
}

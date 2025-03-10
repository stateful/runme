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
	"github.com/stateful/runme/v3/internal/project/projectservice"
	"github.com/stateful/runme/v3/internal/runnerv2client"
	"github.com/stateful/runme/v3/internal/runnerv2service"
	"github.com/stateful/runme/v3/internal/server"
	runmetls "github.com/stateful/runme/v3/internal/tls"
	parserv1 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/parser/v1"
	projectv1 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/project/v1"
	runnerv2 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/runner/v2"
	"github.com/stateful/runme/v3/pkg/document/editor/editorservice"
	"github.com/stateful/runme/v3/pkg/project"
)

var defaultBuilder = NewBuilder()

type Builder struct {
	*dig.Container
}

func NewBuilder() *Builder {
	b := Builder{Container: dig.New()}
	b.init()
	return &b
}

func (b *Builder) init() {
	mustProvide := func(err error) {
		if err != nil {
			panic("failed to provide: " + err.Error())
		}
	}

	container := b

	mustProvide(container.Provide(getClient))
	mustProvide(container.Provide(getClientFactory))
	mustProvide(container.Provide(getCommandFactory))
	mustProvide(container.Provide(getConfigLoader))
	mustProvide(container.Provide(getDocker))
	mustProvide(container.Provide(getLogger))
	mustProvide(container.Provide(getProject))
	mustProvide(container.Provide(getProjectFilters))
	mustProvide(container.Provide(getRootConfig))
	mustProvide(container.Provide(getServer))
}

func Decorate(decorator interface{}, opts ...dig.DecorateOption) error {
	return defaultBuilder.Decorate(decorator, opts...)
}

// Invoke is used to invoke the function with the given dependencies.
// The package will automatically figure out how to instantiate them
// using the available configuration.
func Invoke(function interface{}, opts ...dig.InvokeOption) error {
	err := defaultBuilder.Invoke(function, opts...)
	return dig.RootCause(err)
}

func getClient(cfg *config.Config, logger *zap.Logger) (*runnerv2client.Client, error) {
	if cfg.Server == nil {
		return nil, nil
	}

	opts := []grpc.DialOption{
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(cfg.Server.MaxMessageSize)),
	}

	if tls := cfg.Server.Tls; tls != nil && tls.Enabled {
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

	conn, err := grpc.NewClient(cfg.Server.Address, opts...)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if conn == nil {
		return nil, errors.New("client connection is not configured")
	}
	return runnerv2client.New(conn, logger), nil
}

type ClientFactory func() (*runnerv2client.Client, error)

func getClientFactory(cfg *config.Config, logger *zap.Logger) (ClientFactory, error) {
	return func() (*runnerv2client.Client, error) {
		return getClient(cfg, logger)
	}, nil
}

func getCommandFactory(cfg *config.Config, docker *dockerexec.Docker, logger *zap.Logger, proj *project.Project) command.Factory {
	opts := []command.FactoryOption{
		command.WithDocker(docker),
		command.WithLogger(logger),
		command.WithProject(proj),
		command.WithUseSystemEnv(cfg.Project.Env.UseSystemEnv),
	}
	return command.NewFactory(opts...)
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
		zapConfig.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
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

	if env := c.Project.Env; env != nil {
		opts = append(opts, project.WithEnvFilesReadOrder(env.Sources))
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

func getRootConfig(cfgLoader *config.Loader) (*config.Config, error) {
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

		userCfgDir, err := os.UserConfigDir()
		if err != nil {
			return nil, errors.WithMessage(err, "failed to get user config directory")
		}

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

func getServer(cfg *config.Config, cmdFactory command.Factory, logger *zap.Logger) (*server.Server, error) {
	if cfg.Server == nil {
		return nil, nil
	}

	parserService := editorservice.NewParserServiceServer(logger)
	projectService := projectservice.NewProjectServiceServer(logger)
	runnerService, err := runnerv2service.NewRunnerService(cmdFactory, logger)
	if err != nil {
		return nil, err
	}

	return server.New(
		cfg,
		logger,
		func(sr grpc.ServiceRegistrar) {
			parserv1.RegisterParserServiceServer(sr, parserService)
			projectv1.RegisterProjectServiceServer(sr, projectService)
			runnerv2.RegisterRunnerServiceServer(sr, runnerService)
		},
	)
}

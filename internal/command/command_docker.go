package command

import (
	"context"
	"io"
	"os"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/stateful/runme/v3/internal/config"
	"github.com/stateful/runme/v3/internal/dockerexec"
)

type DockerCommand struct {
	cfg    *Config
	docker *dockerexec.Docker
	opts   Options

	cmd *dockerexec.Cmd

	cleanFuncs []func() error
}

var _ Command = (*DockerCommand)(nil)

func NewDocker(cfg *Config, docker *dockerexec.Docker, opts Options) *DockerCommand {
	if opts.Stdout == nil {
		opts.Stdout = io.Discard
	}

	if opts.Stderr == nil {
		opts.Stderr = io.Discard
	}

	if opts.Logger == nil {
		opts.Logger = zap.NewNop()
	}

	return &DockerCommand{
		cfg:    cfg,
		docker: docker,
		opts:   opts,
	}
}

func (c *DockerCommand) Running() bool {
	return c.cmd != nil && c.cmd.ProcessState == nil
}

func (c *DockerCommand) Pid() int {
	if c.cmd != nil && c.cmd.Process != nil {
		return c.cmd.Process.Pid
	}
	return 0
}

func (c *DockerCommand) Start(ctx context.Context) (err error) {
	if c.opts.Session != nil {
		return errors.New("docker command does not support session")
	}

	// This validation should be in the constructor, but it would mean that
	// all constructors' signatures would have to change. At this point,
	// it's a trade-off to provide a clean API and gracefully handle defaults.
	if c.opts.Kernel == nil {
		c.opts.Kernel, err = NewDockerKernel(
			&config.DockerKernel{
				Image: "alpine:3.19",
			},
			c.opts.Logger,
		)
		if err != nil {
			return err
		}
	}

	cfg := c.cfg

	if cfg.Directory == "" {
		cfg.Directory = "/tmp"
	}

	cfg, cleanups, err := normalizeConfig(
		c.cfg,
		newPathNormalizer(c.opts.Kernel),
		modeNormalizer,
		newArgsNormalizer(c.opts.Session, c.opts.Logger),
		newEnvNormalizer(c.opts.Kernel, c.opts.Session.GetEnv),
	)
	if err != nil {
		return
	}

	c.cleanFuncs = append(c.cleanFuncs, cleanups...)

	cmd := c.docker.CommandContext(
		ctx,
		cfg.ProgramName,
		cfg.Arguments...,
	)
	cmd.Dir = cfg.Directory
	cmd.Env = cfg.Env
	cmd.TTY = true // TODO(adamb): should it be configurable?
	cmd.Stdin = c.opts.Stdin
	cmd.Stdout = c.opts.Stdout
	cmd.Stderr = c.opts.Stderr

	c.cmd = cmd

	c.opts.Logger.Info("starting a docker command", zap.Any("config", redactConfig(cfg)))

	if err := c.cmd.Start(); err != nil {
		return err
	}

	c.opts.Logger.Info("docker command started")

	return nil
}

func (c *DockerCommand) Signal(os.Signal) error {
	return c.cmd.Signal()
}

func (c *DockerCommand) Wait() (err error) {
	c.opts.Logger.Info("waiting for docker command to finish")

	defer func() {
		cleanErr := errors.WithStack(c.cleanup())
		c.opts.Logger.Info("cleaned up the native command", zap.Error(cleanErr))
		if err == nil && cleanErr != nil {
			err = cleanErr
		}
	}()

	err = c.cmd.Wait()

	c.opts.Logger.Info("the docker command finished", zap.Error(err))

	return
}

func (c *DockerCommand) cleanup() (err error) {
	for _, fn := range c.cleanFuncs {
		if errFn := fn(); errFn != nil {
			err = multierr.Append(err, errFn)
		}
	}
	return
}

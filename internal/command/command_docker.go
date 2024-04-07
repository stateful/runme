package command

import (
	"context"
	"io"
	"os"

	"github.com/pkg/errors"
	"github.com/stateful/runme/v3/internal/dockercmd"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

type DockerCommand struct {
	cfg  *Config
	opts *DockerCommandOptions

	cmd *dockercmd.Cmd

	cleanFuncs []func() error

	logger *zap.Logger
}

func newDockerCommand(cfg *Config, opts *DockerCommandOptions) *DockerCommand {
	if opts.Stdout == nil {
		opts.Stdout = io.Discard
	}

	if opts.Stderr == nil {
		opts.Stderr = io.Discard
	}

	return &DockerCommand{
		cfg:    cfg,
		opts:   opts,
		logger: opts.Logger,
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
	cfg := c.cfg

	if cfg.Directory == "" {
		cfg.Directory = "/tmp"
	}

	// TODO(adamb): likely each normalizer should be kernel-dependent.
	// cfg, cleanups, err := normalizeConfig(
	// 	c.cfg,
	// 	newPathNormalizer(),
	// 	modeNormalizer,
	// 	newArgsNormalizer(c.opts.Session, c.logger),
	// 	newEnvNormalizer(c.opts.Session.GetEnv),
	// )
	// if err != nil {
	// 	return
	// }

	// c.cleanFuncs = append(c.cleanFuncs, cleanups...)

	cmd := c.opts.CmdFactory.CommandContext(
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

	c.logger.Info("starting a docker command", zap.Any("config", redactConfig(cfg)))

	if err := c.cmd.Start(); err != nil {
		return err
	}

	c.logger.Info("docker command started")

	return nil
}

func (c *DockerCommand) StopWithSignal(sig os.Signal) error {
	return c.cmd.Signal()
}

func (c *DockerCommand) Wait() (err error) {
	c.logger.Info("waiting for docker command to finish")

	defer func() {
		cleanErr := errors.WithStack(c.cleanup())
		c.logger.Info("cleaned up the native command", zap.Error(cleanErr))
		if err == nil && cleanErr != nil {
			err = cleanErr
		}
	}()

	err = c.cmd.Wait()

	c.logger.Info("the docker command finished", zap.Error(err))

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

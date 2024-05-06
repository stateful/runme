package command

import (
	"context"
	"os"

	"go.uber.org/zap"

	"github.com/stateful/runme/v3/internal/dockerexec"
)

type dockerCommand struct {
	internalCommand

	cmd    *dockerexec.Cmd
	docker *dockerexec.Docker
}

var _ Command = (*dockerCommand)(nil)

func newDocker(docker *dockerexec.Docker, cfg *ProgramConfig, opts Options) *dockerCommand {
	if opts.Kernel == nil {
		opts.Kernel = NewDockerKernel(docker)
	}
	return &dockerCommand{
		internalCommand: newBase(cfg, opts),
		docker:          docker,
	}
}

func (c *dockerCommand) Running() bool {
	return c.cmd != nil && c.cmd.ProcessState == nil
}

func (c *dockerCommand) Pid() int {
	if c.cmd == nil || c.cmd.Process == nil {
		return 0
	}
	return c.cmd.Process.Pid
}

func (c *dockerCommand) Start(ctx context.Context) (err error) {
	logger := c.Logger()

	cfg := c.ProgramConfig()
	if cfg.Directory == "" {
		cfg.Directory = "/tmp"
	}

	program, args, err := c.ProgramPath()
	if err != nil {
		return err
	}

	cmd := c.docker.CommandContext(
		ctx,
		program,
		args...,
	)
	cmd.Dir = cfg.Directory
	cmd.Env = c.Env()
	cmd.TTY = true // TODO(adamb): should it be configurable?
	cmd.Stdin = c.Stdin()
	cmd.Stdout = c.Stdout()
	cmd.Stderr = c.Stderr()

	c.cmd = cmd

	logger.Info("starting a docker command", zap.Any("config", redactConfig(cfg)))
	if err := c.cmd.Start(); err != nil {
		return err
	}
	logger.Info("docker command started")

	return nil
}

func (c *dockerCommand) Signal(os.Signal) error {
	return c.cmd.Signal()
}

func (c *dockerCommand) Wait() (err error) {
	c.Logger().Info("waiting for the docker command to finish")
	err = c.cmd.Wait()
	c.Logger().Info("the docker command finished", zap.Error(err))
	return
}

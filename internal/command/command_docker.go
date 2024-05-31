package command

import (
	"context"
	"os"

	"go.uber.org/zap"

	"github.com/stateful/runme/v3/internal/dockerexec"
)

type dockerCommand struct {
	*base

	cmd    *dockerexec.Cmd
	docker *dockerexec.Docker
	logger *zap.Logger
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

	c.logger.Info("starting a docker command", zap.Any("config", redactConfig(cfg)))
	if err := c.cmd.Start(); err != nil {
		return err
	}
	c.logger.Info("docker command started")

	return nil
}

func (c *dockerCommand) Signal(os.Signal) error {
	return c.cmd.Signal()
}

func (c *dockerCommand) Wait() (err error) {
	c.logger.Info("waiting for the docker command to finish")
	err = c.cmd.Wait()
	c.logger.Info("the docker command finished", zap.Error(err))
	return
}

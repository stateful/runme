package command

import (
	"context"
	"os"
)

type DockerCommand struct {
	cfg  *Config
	opts *DockerCommandOptions

	pid     int
	running bool
}

func newDockerCommand(cfg *Config, options *DockerCommandOptions) *DockerCommand {
	return &DockerCommand{
		cfg:  cfg,
		opts: options,
	}
}

func (c *DockerCommand) Running() bool {
	return c.running
}

func (c *DockerCommand) Pid() int {
	return c.pid
}

func (c *DockerCommand) Start(ctx context.Context) error {
	// 1. ContainerExecCreate
	// 2. ContainerExecAttach
	// 3. ContainerExecStart
	// 4. ContainerExecInspect

	// cfg, cleanups, err := normalizeConfig(
	// 	c.cfg,
	// 	pathNormalizer, // TODO(adamb): should use kernel for looking paths
	// 	modeNormalizer,
	// 	newArgsNormalizer(c.opts.Session, c.logger), // TODO(adamb): should use kernel for creating temp files
	// 	newEnvNormalizer(c.opts.Session.GetEnv),     // TODO(adamb): should use kernel for OS environ
	// )
	// if err != nil {
	// 	return
	// }

	// c.opts.Kernel.Exec(ctx, ...)

	return nil
}

func (c *DockerCommand) StopWithSignal(sig os.Signal) error {
	return nil
}

func (c *DockerCommand) Wait() error {
	return nil
}

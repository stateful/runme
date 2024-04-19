package command

import (
	"context"

	"github.com/pkg/errors"
)

type TerminalCommand struct {
	*VirtualCommand

	envCollector *shellEnvCollector
}

var _ Command = (*TerminalCommand)(nil)

func NewTerminal(cfg *Config, opts Options) *TerminalCommand {
	return &TerminalCommand{
		VirtualCommand: NewVirtual(cfg, opts),
	}
}

func (c *TerminalCommand) Start(ctx context.Context) error {
	if isNil(c.opts.StdinWriter) {
		return errors.New("stdin writer is nil")
	}

	if err := c.VirtualCommand.Start(ctx); err != nil {
		return err
	}

	c.envCollector = &shellEnvCollector{
		buf: c.opts.StdinWriter,
	}
	return c.envCollector.Init()
}

func (c *TerminalCommand) Wait() (err error) {
	err = c.VirtualCommand.Wait()

	if cErr := c.collectEnv(); err == nil && cErr != nil {
		err = cErr
	}

	return err
}

func (c *TerminalCommand) collectEnv() error {
	if c.opts.Session == nil || c.envCollector == nil {
		return nil
	}

	changed, deleted, err := c.envCollector.Collect()
	if err != nil {
		return err
	}

	if err := c.opts.Session.SetEnv(changed...); err != nil {
		return errors.WithMessage(err, "failed to set the new or updated env")
	}

	c.opts.Session.DeleteEnv(deleted...)

	return nil
}

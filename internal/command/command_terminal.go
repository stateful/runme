package command

import (
	"context"
	"io"
	"os"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type terminalCommand struct {
	internalCommand

	envCollector envCollector
	logger       *zap.Logger
	session      *Session
	stdinWriter  io.Writer
}

func (c *terminalCommand) getPty() *os.File {
	cmdPty, ok := c.internalCommand.(commandWithPty)
	if !ok {
		return nil
	}
	return cmdPty.getPty()
}

func (c *terminalCommand) Start(ctx context.Context) (err error) {
	if isNil(c.stdinWriter) {
		return errors.New("stdin writer is nil")
	}

	cfg := c.ProgramConfig()
	if c.envCollector != nil {
		cfg.Env = append(cfg.Env, c.envCollector.ExtraEnv()...)
	}

	c.logger.Info("starting a terminal command")
	if err := c.internalCommand.Start(ctx); err != nil {
		return err
	}
	c.logger.Info("a terminal command started")

	if c.envCollector != nil {
		if err := c.envCollector.SetOnShell(c.stdinWriter); err != nil {
			return err
		}
	}

	if _, err := c.stdinWriter.Write([]byte(" eval $(runme beta env source --silent --insecure --export)\n clear\n")); err != nil {
		return err
	}

	// todo(sebastian): good enough for prototype; it makes more sense to write this message at the TTY-level
	initMsg := []byte(" # Runme: This terminal forked your session. " +
		"Upon exit exported environment variables will be rolled up into the session.\n\n")
	_, err = c.stdinWriter.Write(initMsg)

	return err
}

func (c *terminalCommand) Wait() (err error) {
	err = c.internalCommand.Wait()
	if cErr := c.collectEnv(); cErr != nil {
		c.logger.Info("failed to collect the environment", zap.Error(cErr))
		if err == nil {
			err = cErr
		}
	}
	return err
}

func (c *terminalCommand) collectEnv() error {
	if c.envCollector == nil {
		return nil
	}

	changed, deleted, err := c.envCollector.Diff()
	if err != nil {
		return err
	}

	err = c.session.SetEnv(changed...)
	if err != nil {
		return errors.WithMessage(err, "failed to set the new or updated env")
	}

	c.session.DeleteEnv(deleted...)

	return nil
}

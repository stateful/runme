//go:build !windows
// +build !windows

package command

import (
	"io"
	"syscall"

	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

func buildShellEnvCollector(w io.Writer) (shellEnvCollector, error) {
	if useFifoShellEnvCollector {
		return newFifoShellEnvCollector(w)
	}
	return newFileShellEnvCollector(w)
}

type fifoShellEnvCollector struct {
	*fsShellEnvCollector
	startEnv     []string
	exitEnv      []string
	readersGroup *errgroup.Group
}

func newFifoShellEnvCollector(w io.Writer) (*fifoShellEnvCollector, error) {
	c := &fifoShellEnvCollector{
		fsShellEnvCollector: &fsShellEnvCollector{
			onStartFile: "env_start.fifo",
			onExitFile:  "env_end.fifo",
			w:           w,
		},
		readersGroup: new(errgroup.Group),
	}
	if err := c.init(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *fifoShellEnvCollector) init() error {
	err := c.fsShellEnvCollector.init()
	if err != nil {
		return err
	}

	err = c.createFifo(c.onStartPath())
	if err != nil {
		return errors.WithMessage(err, "failed to create the start FIFO")
	}

	err = c.createFifo(c.onExitPath())
	if err != nil {
		return errors.WithMessage(err, "failed to create the exit FIFO")
	}

	c.readersGroup.Go(func() (err error) {
		c.startEnv, err = c.read(c.onStartPath())
		return err
	})

	c.readersGroup.Go(func() (err error) {
		c.exitEnv, err = c.read(c.onExitPath())
		return err
	})

	return nil
}

func (c *fifoShellEnvCollector) Collect() (changed, deleted []string, err error) {
	defer func() {
		if cErr := c.cleanup(); cErr != nil && err == nil {
			err = cErr
		}
	}()

	if err := c.readersGroup.Wait(); err != nil {
		return nil, nil, err
	}

	return c.diff(c.startEnv, c.exitEnv)
}

func (c *fifoShellEnvCollector) createFifo(name string) error {
	err := syscall.Mkfifo(name, 0o600)
	return errors.WithMessage(err, "failed to create a FIFO")
}

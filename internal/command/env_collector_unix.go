package command

import (
	"syscall"

	"github.com/pkg/errors"
)

type envCollectorFifoStorer struct {
	*tempDirectory
}

var _ envCollectorStorer = (*envCollectorFifoStorer)(nil)

func newEnvCollectorFifoStorer() (*envCollectorFifoStorer, error) {
	temp, err := newTempDirectory()
	if err != nil {
		return nil, err
	}
	storer := &envCollectorFifoStorer{tempDirectory: temp}
	if storer.init() != nil {
		return nil, err
	}
	return storer, nil
}

func (c *envCollectorFifoStorer) init() (err error) {
	defer func() {
		if err != nil {
			_ = c.tempDirectory.Cleanup()
		}
	}()

	err = c.createFifo(c.PrePath())
	if err != nil {
		err = errors.WithMessagef(errFifoCreate, "failed to create the start FIFO: %s", err)
		return
	}

	err = c.createFifo(c.PostPath())
	if err != nil {
		err = errors.WithMessagef(errFifoCreate, "failed to create the exit FIFO: %s", err)
		return
	}

	return
}

func (c *envCollectorFifoStorer) PrePath() string {
	return c.tempDirectory.Join("env_pre.fifo")
}

func (c *envCollectorFifoStorer) PostPath() string {
	return c.tempDirectory.Join("env_post.fifo")
}

func (*envCollectorFifoStorer) createFifo(name string) error {
	err := syscall.Mkfifo(name, 0o600)
	return errors.WithMessage(err, "failed to create a FIFO")
}

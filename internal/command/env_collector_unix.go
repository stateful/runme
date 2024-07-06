//go:build !windows
// +build !windows

package command

import (
	"io"
	"syscall"

	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

var errFifoCreate = errors.New("failed to create a fifo")

type envCollectorFifo struct {
	*tempDirectory

	preEnv       []string
	postEnv      []string
	readersGroup *errgroup.Group
	scanner      envScanner
}

func newEnvCollectorFifo(scanner envScanner) (*envCollectorFifo, error) {
	temp, err := newTempDirectory()
	if err != nil {
		return nil, err
	}

	c := &envCollectorFifo{
		tempDirectory: temp,
		readersGroup:  new(errgroup.Group),
		scanner:       scanner,
	}

	if c.init() != nil {
		return nil, err
	}

	return c, nil
}

func (c *envCollectorFifo) init() (err error) {
	err = c.createFifo(c.prePath())
	if err != nil {
		err = errors.WithMessagef(errFifoCreate, "failed to create the start fifo: %s", err)
		return
	}

	err = c.createFifo(c.postPath())
	if err != nil {
		err = errors.WithMessagef(errFifoCreate, "failed to create the exit fifo: %s", err)
		return
	}

	c.readersGroup.Go(func() error {
		r, err := c.open(c.prePath())
		if err != nil {
			return err
		}
		defer r.Close()
		c.preEnv, err = scanEnv(r)
		return err
	})

	c.readersGroup.Go(func() error {
		r, err := c.open(c.postPath())
		if err != nil {
			return err
		}
		defer r.Close()
		c.postEnv, err = c.scanner(r)
		return err
	})

	return
}

func (c *envCollectorFifo) Diff() (changed []string, deleted []string, _ error) {
	defer c.cleanup()
	if err := c.readersGroup.Wait(); err != nil {
		return nil, nil, err
	}
	return diffEnvs(c.preEnv, c.postEnv)
}

func (c *envCollectorFifo) SetOnShell(shell io.Writer) error {
	return setOnShell(shell, c.prePath(), c.postPath())
}

func (c *envCollectorFifo) prePath() string {
	return c.join("env_pre.fifo")
}

func (c *envCollectorFifo) postPath() string {
	return c.join("env_post.fifo")
}

func (*envCollectorFifo) createFifo(name string) error {
	err := syscall.Mkfifo(name, 0o600)
	return errors.WithMessage(err, "failed to create a FIFO")
}

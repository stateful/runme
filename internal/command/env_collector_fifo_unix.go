//go:build !windows
// +build !windows

package command

import (
	"encoding/hex"
	"io"
	"syscall"

	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

type envCollectorFifo struct {
	encKey       []byte
	encNonce     []byte
	preEnv       []string
	postEnv      []string
	readersGroup *errgroup.Group
	scanner      envScanner
	temp         *tempDirectory
}

func newEnvCollectorFifo(scanner envScanner, encKey, encNonce []byte) (*envCollectorFifo, error) {
	temp, err := newTempDirectory()
	if err != nil {
		return nil, err
	}

	c := &envCollectorFifo{
		encKey:       encKey,
		encNonce:     encNonce,
		readersGroup: new(errgroup.Group),
		scanner:      scanner,
		temp:         temp,
	}

	if c.init() != nil {
		return nil, err
	}

	return c, nil
}

func (c *envCollectorFifo) init() error {
	err := c.createFifo(c.prePath())
	if err != nil {
		return errors.Wrap(err, "failed to create the pre-execute fifo")
	}

	err = c.createFifo(c.postPath())
	if err != nil {
		return errors.Wrap(err, "failed to create the post-exit fifo")
	}

	c.readersGroup.Go(func() error {
		var err error
		c.preEnv, err = c.read(c.prePath())
		return err
	})

	c.readersGroup.Go(func() error {
		var err error
		c.postEnv, err = c.read(c.postPath())
		return err
	})

	return nil
}

func (c *envCollectorFifo) Diff() (changed []string, deleted []string, _ error) {
	defer c.temp.Cleanup()
	if err := c.readersGroup.Wait(); err != nil {
		return nil, nil, err
	}
	return diffEnvs(c.preEnv, c.postEnv)
}

func (c *envCollectorFifo) ExtraEnv() []string {
	if c.encKey == nil || c.encNonce == nil {
		return nil
	}
	return []string{
		"RUNME_ENCRYPTION_KEY=" + hex.EncodeToString(c.encKey),
		"RUNME_ENCRYPTION_NONCE=" + hex.EncodeToString(c.encNonce),
	}
}

func (c *envCollectorFifo) SetOnShell(shell io.Writer) error {
	return setOnShell(shell, c.prePath(), c.postPath())
}

func (c *envCollectorFifo) prePath() string {
	return c.temp.Join("env_pre.fifo")
}

func (c *envCollectorFifo) postPath() string {
	return c.temp.Join("env_post.fifo")
}

func (*envCollectorFifo) createFifo(name string) error {
	err := syscall.Mkfifo(name, 0o600)
	return errors.WithMessage(err, "failed to create a FIFO")
}

func (c *envCollectorFifo) read(path string) ([]string, error) {
	r, err := c.temp.Open(path)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return c.scanner(r)
}

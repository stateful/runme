//go:build !windows
// +build !windows

package command

import (
	"encoding/hex"
	"io"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"golang.org/x/sync/errgroup"
)

var sentinel = []byte("\x00")

type envCollectorFifo struct {
	encKey       []byte
	encNonce     []byte
	preEnv       []string
	postEnv      []string
	readersDone  map[string]chan struct{}
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
		encKey:   encKey,
		encNonce: encNonce,
		scanner:  scanner,
		temp:     temp,
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

	c.readersDone = map[string]chan struct{}{
		c.prePath():  make(chan struct{}),
		c.postPath(): make(chan struct{}),
	}
	c.readersGroup = &errgroup.Group{}

	c.readersGroup.Go(func() error {
		var err error
		c.preEnv, err = c.read(c.prePath())
		close(c.readersDone[c.prePath()])
		return err
	})

	c.readersGroup.Go(func() error {
		var err error
		c.postEnv, err = c.read(c.postPath())
		close(c.readersDone[c.postPath()])
		return err
	})

	return nil
}

func (c *envCollectorFifo) Diff() (changed []string, deleted []string, _ error) {
	defer c.cleanup()

	errc := make(chan error, 3)

	go func() {
		err := c.ensureReaderDone(c.prePath())
		errc <- err
	}()

	go func() {
		err := c.ensureReaderDone(c.postPath())
		errc <- err
	}()

	go func() {
		err := c.readersGroup.Wait()
		errc <- err
	}()

	var err error
	for i := 0; i < 3; i++ {
		err = multierr.Append(err, <-errc)
	}
	if err != nil {
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

func (c *envCollectorFifo) cleanup() error {
	return c.temp.Cleanup()
}

func (*envCollectorFifo) createFifo(name string) error {
	err := syscall.Mkfifo(name, 0o600)
	return errors.WithMessage(err, "failed to create a FIFO")
}

func (*envCollectorFifo) open(name string) (*os.File, error) {
	f, err := os.OpenFile(name, os.O_RDONLY, 0o600)
	if err != nil {
		return nil, errors.WithMessagef(err, "failed to open (read-only) fifo %q", name)
	}
	return f, nil
}

func (c *envCollectorFifo) read(path string) ([]string, error) {
	r, err := c.open(path)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return c.scanner(r)
}

func (c *envCollectorFifo) ensureReaderDone(path string) error {
	for {
		select {
		case <-c.readersDone[path]:
			return nil
		case <-time.After(time.Millisecond * 100):
			err := c.writeSentinel(path)
			if err != nil {
				if errors.Is(err, errFifoNotAvailable) {
					continue
				}
				return err
			}
			return nil
		}
	}
}

var errFifoNotAvailable = errors.New("fifo not available")

func (c *envCollectorFifo) writeSentinel(name string) error {
	f, err := os.OpenFile(name, os.O_WRONLY|syscall.O_NONBLOCK, 0o600)
	if err != nil {
		if strings.Contains(err.Error(), "device not configured") {
			// The FIFO is not opened for reading yet, or it was already closed.
			// This is expected when writing a sentinel and we can ignore the error.
			return errFifoNotAvailable
		}
		return errors.WithStack(err)
	}
	defer f.Close()
	_, _ = f.Write(sentinel)
	return nil
}

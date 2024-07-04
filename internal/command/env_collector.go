package command

import (
	"bufio"
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

type envCollectorStorer interface {
	Cleanup() error
	Open(string) (io.ReadCloser, error)
	PrePath() string
	PostPath() string
}

type envCollector struct {
	storer envCollectorStorer
}

func newEnvCollector(s envCollectorStorer) *envCollector {
	return &envCollector{storer: s}
}

func (c *envCollector) Diff() (changed []string, deleted []string, _ error) {
	initial, err := c.collect(c.storer.PrePath())
	if err != nil {
		return nil, nil, err
	}

	final, err := c.collect(c.storer.PostPath())
	if err != nil {
		return nil, nil, err
	}

	return diffEnvs(initial, final)
}

func (c *envCollector) collect(path string) ([]string, error) {
	r, err := c.storer.Open(path)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to open the env file")
	}
	defer r.Close()
	return c.scan(r)
}

func (c *envCollector) scan(r io.Reader) (result []string, err error) {
	scanner := bufio.NewScanner(r)
	scanner.Split(splitNull)

	for scanner.Scan() {
		result = append(result, scanner.Text())
	}

	err = errors.Wrap(scanner.Err(), "failed to scan env stream")
	if err != nil {
		return
	}

	return result, nil
}

func diffEnvs(initial, final []string) (changed, deleted []string, err error) {
	envStoreWithInitial := newEnvStore()
	_, err = envStoreWithInitial.Merge(initial...)
	if err != nil {
		return nil, nil, errors.WithMessage(err, "failed to create the store with initial env")
	}

	envStoreWithFinal := newEnvStore()
	_, err = envStoreWithFinal.Merge(final...)
	if err != nil {
		return nil, nil, errors.WithMessage(err, "failed to create the store with final env")
	}

	changed, _, deleted = diffEnvStores(envStoreWithInitial, envStoreWithFinal)
	return
}

type envCollectorFileStorer struct {
	*tempDirectory
}

var _ envCollectorStorer = (*envCollectorFileStorer)(nil)

func newEnvCollectorFileStorer() (*envCollectorFileStorer, error) {
	temp, err := newTempDirectory()
	if err != nil {
		return nil, err
	}
	return &envCollectorFileStorer{tempDirectory: temp}, nil
}

func (c *envCollectorFileStorer) PrePath() string {
	return c.Join(".env_pre")
}

func (c *envCollectorFileStorer) PostPath() string {
	return c.Join(".env_post")
}

type tempDirectory struct {
	dir string
}

func newTempDirectory() (*tempDirectory, error) {
	c := &tempDirectory{}
	if err := c.ensureTempDir(); err != nil {
		return nil, err
	}
	return c, nil
}

func (m *tempDirectory) Cleanup() error {
	return m.removeTempDir()
}

func (m *tempDirectory) Join(name string) string {
	return filepath.Join(m.dir, name)
}

func (*tempDirectory) Open(name string) (io.ReadCloser, error) {
	f, err := os.OpenFile(name, os.O_RDONLY, 0o600)
	if err != nil {
		return nil, errors.WithMessagef(err, "failed to open the env file %q", name)
	}
	return f, nil
}

func (m *tempDirectory) ensureTempDir() error {
	info, err := os.Stat(m.dir)
	if err != nil && !os.IsNotExist(err) {
		return errors.WithMessage(err, "failed to check the temporary dir")
	}
	if err != nil && os.IsNotExist(err) {
		return m.createTempDir()
	}
	if !info.IsDir() {
		return errors.New("the temporary dir is not a directory")
	}
	return nil
}

func (m *tempDirectory) createTempDir() (err error) {
	m.dir, err = os.MkdirTemp("", "runme-*")
	if err != nil {
		return errors.WithMessage(err, "failed to create a temporary dir")
	}
	return nil
}

func (m *tempDirectory) removeTempDir() error {
	err := os.RemoveAll(m.dir)
	return errors.WithMessage(err, "failed to remove the temporary dir")
}

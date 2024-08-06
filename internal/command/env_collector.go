package command

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

const maxScannerBufferSizeInBytes = 1024 * 1024 * 1024 // 1GB

const (
	envCollectorEncKeyEnvName   = "RUNME_ENCRYPTION_KEY"
	envCollectorEncNonceEnvName = "RUNME_ENCRYPTION_NONCE"
)

// envDumpCommand is a command that dumps the environment variables.
var envDumpCommand = func() string {
	path, err := os.Executable()
	if err != nil {
		panic(errors.WithMessage(err, "failed to get the executable path"))
	}
	return strings.Join([]string{path, "env", "dump", "--insecure"}, " ")
}()

func SetEnvDumpCommand(cmd string) {
	envDumpCommand = cmd
	// When overriding [envDumpCommand], we disable the encryption.
	// There is no way to test the encryption if the dump command
	// is not controlled.
	envCollectorEnableEncryption = false
}

type envCollector interface {
	// Diff compares the environment variables before and after the command execution.
	// It returns the list of env that were changed and deleted.
	Diff() (changed []string, deleted []string, _ error)

	// ExtraEnv provides a list of extra environment variables that should be set
	// before the command execution.
	ExtraEnv() []string

	// SetOnShell writes additional commands to the shell session
	// in order to collect the environment variables after
	// the command execution.
	SetOnShell(io.Writer) error
}

type envScanner func(io.Reader) ([]string, error)

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

func scanEnv(r io.Reader) ([]string, error) {
	scanner := bufio.NewScanner(r)
	// 4096 is taken from bufio as the initial buffer size.
	scanner.Buffer(make([]byte, 4096), int(maxScannerBufferSizeInBytes))
	scanner.Split(splitNull)

	var result []string
	for scanner.Scan() {
		result = append(result, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, errors.Wrap(err, "failed to scan env stream")
	}
	return result, nil
}

func splitNull(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, byte(0)); i >= 0 {
		// We have a full null-terminated line.
		return i + 1, data[0:i], nil
	}
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), data, nil
	}
	// Request more data.
	return 0, nil, nil
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

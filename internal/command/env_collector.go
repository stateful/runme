package command

import (
	"bufio"
	"bytes"
	"encoding/hex"
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

// EnvDumpCommand is a command that dumps the environment variables.
// It is declared as a var, because it must be replaced in tests.
// Equivalent is `env -0`.
var EnvDumpCommand = func() string {
	path, err := os.Executable()
	if err != nil {
		panic(errors.WithMessage(err, "failed to get the executable path"))
	}
	return strings.Join([]string{path, "env", "dump", "--insecure"}, " ")
}()

type envCollectorFactoryOptions struct {
	encryptionEnabled bool
	useFifo           bool
}

type envCollectorFactory struct {
	opts envCollectorFactoryOptions
}

func newEnvCollectorFactory(opts envCollectorFactoryOptions) *envCollectorFactory {
	return &envCollectorFactory{
		opts: opts,
	}
}

func (f *envCollectorFactory) Build() (envCollector, error) {
	scanner := scanEnv

	var (
		encKey   []byte
		encNonce []byte
	)

	if f.opts.encryptionEnabled {
		var err error
		encKey, encNonce, err = f.generateEncryptionKeyAndNonce()
		if err != nil {
			return nil, err
		}
	}

	if f.opts.encryptionEnabled {
		scannerPrev := scanner

		scanner = func(r io.Reader) ([]string, error) {
			enc, err := NewEnvDecryptor(encKey, encNonce, r)
			if err != nil {
				return nil, err
			}
			return scannerPrev(enc)
		}
	}

	if f.opts.useFifo {
		return newEnvCollectorFifo(scanner, encKey, encNonce)
	}

	return newEnvCollectorFile(scanner, encKey, encNonce)
}

func (f *envCollectorFactory) generateEncryptionKeyAndNonce() ([]byte, []byte, error) {
	encKey, err := CreateKey()
	if err != nil {
		return nil, nil, errors.WithMessage(err, "failed to create the encryption key")
	}

	encNonce, err := CreateKey()
	if err != nil {
		return nil, nil, errors.WithMessage(err, "failed to create the encryption nonce")
	}

	return encKey, encNonce, nil
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

type envCollectorFile struct {
	*tempDirectory

	encKey   []byte
	encNonce []byte
	scanner  envScanner
}

var _ envCollector = (*envCollectorFile)(nil)

func newEnvCollectorFile(
	scanner envScanner,
	encKey []byte,
	encNonce []byte,
) (*envCollectorFile, error) {
	temp, err := newTempDirectory()
	if err != nil {
		return nil, err
	}

	return &envCollectorFile{
		tempDirectory: temp,
		encKey:        encKey,
		encNonce:      encNonce,
		scanner:       scanner,
	}, nil
}

func (c *envCollectorFile) Diff() (changed []string, deleted []string, _ error) {
	defer c.cleanup()

	initialReader, err := c.open(c.prePath())
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = initialReader.Close() }()

	initial, err := c.scanner(initialReader)
	if err != nil {
		return nil, nil, err
	}

	finalReader, err := c.open(c.postPath())
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = finalReader.Close() }()

	final, err := c.scanner(finalReader)
	if err != nil {
		return nil, nil, err
	}

	return diffEnvs(initial, final)
}

func (c *envCollectorFile) ExtraEnv() []string {
	return []string{
		envCollectorEncKeyEnvName + "=" + hex.EncodeToString(c.encKey),
		envCollectorEncNonceEnvName + "=" + hex.EncodeToString(c.encNonce),
	}
}

func (c *envCollectorFile) SetOnShell(shell io.Writer) error {
	return setOnShell(shell, c.prePath(), c.postPath())
}

func (c *envCollectorFile) prePath() string {
	return c.join(".env_pre")
}

func (c *envCollectorFile) postPath() string {
	return c.join(".env_post")
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
	if i := bytes.IndexByte(data, 0); i >= 0 {
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

func (m *tempDirectory) cleanup() error {
	return m.removeTempDir()
}

func (m *tempDirectory) join(name string) string {
	return filepath.Join(m.dir, name)
}

func (*tempDirectory) open(name string) (io.ReadCloser, error) {
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

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

var (
	// EnvDumpCommand is a command that dumps the environment variables.
	// It is declared as a var, because it must be replaced in tests.
	// Equivalent is `env -0`.
	EnvDumpCommand = func() string {
		path, err := os.Executable()
		if err != nil {
			panic(errors.WithMessage(err, "failed to get the executable path"))
		}
		return strings.Join([]string{path, "env", "dump", "--insecure"}, " ")
	}()

	errFifoCreate = errors.New("failed to create fifo")
)

type shellEnvCollectorFactory struct {
	useFifo bool
}

func newShellEnvCollectorFactory(useFifo bool) *shellEnvCollectorFactory {
	return &shellEnvCollectorFactory{useFifo: useFifo}
}

func (f *shellEnvCollectorFactory) Build(w io.Writer) (shellEnvCollector, error) {
	if f.useFifo {
		collector, err := newFifoShellEnvCollector(w)
		if err == nil {
			return collector, nil
		}
		if !errors.Is(err, errFifoCreate) {
			return nil, err
		}
		// If the FIFO collector failed to create the FIFOs,
		// fallback to the file-based collector.
	}
	return newFileShellEnvCollector(w)
}

// shellEnvCollector collects the environment variables from a shell script or session (terminal mode).
// It writes a shell command that dumps the initial environment variables. Then, it configures a trap
// on the EXIT signal which will dump the environment variables again. Finally, it compares the two
// dumps to find the changed and deleted variables.
type shellEnvCollector interface {
	Collect() (changed, deleted []string, _ error)
}

type fileShellEnvCollector struct {
	*fsShellEnvCollector
}

func newFileShellEnvCollector(w io.Writer) (*fileShellEnvCollector, error) {
	c := &fileShellEnvCollector{
		fsShellEnvCollector: &fsShellEnvCollector{
			onStartFile: ".env_start",
			onExitFile:  ".env_end",
			w:           w,
		},
	}
	if err := c.init(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *fileShellEnvCollector) Collect() (changed, deleted []string, err error) {
	defer func() {
		if cErr := c.cleanup(); cErr != nil && err == nil {
			err = cErr
		}
	}()

	startEnv, err := c.read(c.onStartPath())
	if err != nil {
		return
	}

	endEnv, err := c.read(c.onExitPath())
	if err != nil {
		return
	}

	changed, deleted, err = c.diff(startEnv, endEnv)
	return
}

type fsShellEnvCollector struct {
	onStartFile   string
	onExitFile    string
	owningTempDir bool
	tempDir       string    // dir in which to create fifos
	w             io.Writer // where to write the shell commands
}

func (c *fsShellEnvCollector) init() error {
	err := c.ensureTempDir()
	if err != nil {
		return err
	}
	// First, dump all env at the beginning, so that a diff can be calculated.
	_, err = c.w.Write([]byte(EnvDumpCommand + " > " + c.onStartPath() + "\n"))
	if err != nil {
		return err
	}
	// Then, set a trap on EXIT to dump all env at the end.
	_, err = c.setTrap(EnvDumpCommand + " > " + c.onExitPath() + "\n")
	return err
}

func (c *fsShellEnvCollector) cleanup() error {
	return c.removeTempDir()
}

func (c *fsShellEnvCollector) diff(startEnv, endEnv []string) (changed, deleted []string, err error) {
	startEnvStore := newEnvStore()
	_, err = startEnvStore.Merge(startEnv...)
	if err != nil {
		err = errors.WithMessage(err, "failed to create the start env store")
		return
	}

	endEnvStore := newEnvStore()
	_, err = endEnvStore.Merge(endEnv...)
	if err != nil {
		err = errors.WithMessage(err, "failed to create the end env store")
		return
	}

	changed, _, deleted = diffEnvStores(startEnvStore, endEnvStore)
	return
}

func (c *fsShellEnvCollector) read(name string) ([]string, error) {
	f, err := os.OpenFile(name, os.O_RDONLY, 0o600)
	if err != nil {
		return nil, errors.WithMessagef(err, "failed to open the env file %q", name)
	}
	defer f.Close()
	return c.scan(f)
}

func (c *fsShellEnvCollector) scan(r io.Reader) (result []string, _ error) {
	scanner := bufio.NewScanner(r)
	scanner.Split(splitNull)

	for scanner.Scan() {
		result = append(result, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, errors.WithMessage(err, "failed to scan the env")
	}

	return result, nil
}

func (c *fsShellEnvCollector) ensureTempDir() error {
	info, err := os.Stat(c.tempDir)
	if err != nil && !os.IsNotExist(err) {
		return errors.WithMessage(err, "failed to check the temporary dir")
	}
	if err != nil && os.IsNotExist(err) {
		return c.createTempDir()
	}
	if !info.IsDir() {
		return errors.New("the temporary dir is not a directory")
	}
	return nil
}

func (c *fsShellEnvCollector) createTempDir() (err error) {
	c.tempDir, err = os.MkdirTemp("", "runme-*")
	if err != nil {
		return errors.WithMessage(err, "failed to create a temporary dir")
	}

	c.owningTempDir = true

	return nil
}

func (c *fsShellEnvCollector) removeTempDir() error {
	if !c.owningTempDir {
		return nil
	}
	err := os.RemoveAll(c.tempDir)
	return errors.WithMessage(err, "failed to remove the temporary dir")
}

func (c *fsShellEnvCollector) onStartPath() string {
	return filepath.Join(c.tempDir, c.onStartFile)
}

func (c *fsShellEnvCollector) onExitPath() string {
	return filepath.Join(c.tempDir, c.onExitFile)
}

func (c *fsShellEnvCollector) setTrap(cmd string) (int, error) {
	bw := bulkWriter{Writer: c.w}
	bw.Write([]byte("__cleanup() {\nrv=$?\n" + cmd + "\nexit $rv\n}\n"))
	bw.Write([]byte("trap -- \"__cleanup\" EXIT\n"))
	return bw.Done()
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

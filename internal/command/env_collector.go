package command

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

const (
	envStartFileName = ".env_start"
	envEndFileName   = ".env_end"

	maxScannerBufferSizeInBytes = 1024 * 1024 * 1024
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

// shellEnvCollector collects the environment variables from a shell script or session.
// It writes a shell command that dumps the initial environment variables into a file.
// Then, it configures a trap on the EXIT signal which will dump the environment variables again
// on exit.
//
// TODO(adamb): change the implementation to use tmpfs.
type shellEnvCollector struct {
	buf         io.Writer // where to write the shell commands
	collectable bool      // true only if the collector is initialized
	tempDir     string    // where to store the env files

	owningTempDir bool
}

func (c *shellEnvCollector) Init() error {
	info, err := os.Stat(c.tempDir)
	if err != nil && !os.IsNotExist(err) {
		return errors.WithMessage(err, "failed to check the temporary dir")
	} else if err != nil && os.IsNotExist(err) {
		if err := c.createTempDir(); err != nil {
			return err
		}
	} else if !info.IsDir() {
		return errors.New("the temporary dir is not a directory")
	}

	// First, dump all env at the beginning, so that a diff can be calculated.
	_, err = c.buf.Write(
		[]byte(
			fmt.Sprintf("%s > %s\n", EnvDumpCommand, filepath.Join(c.tempDir, envStartFileName)),
		),
	)
	if err != nil {
		return err
	}
	// Then, set a trap on EXIT to dump all env at the end.
	_, err = setEnvDumpTrapOnExit(
		c.buf,
		fmt.Sprintf("%s > %s", EnvDumpCommand, filepath.Join(c.tempDir, envEndFileName)),
	)
	if err != nil {
		return err
	}

	c.collectable = true

	return nil
}

func (c *shellEnvCollector) Collect() (changed, deleted []string, _ error) {
	if !c.collectable {
		return nil, nil, nil
	}

	defer func() {
		_ = c.removeTempDir()
	}()

	startEnv, err := c.readEnvFromFile(envStartFileName)
	if err != nil {
		return nil, nil, err
	}

	endEnv, err := c.readEnvFromFile(envEndFileName)
	if err != nil {
		return nil, nil, err
	}

	startEnvStore := newEnvStore()
	if _, err := startEnvStore.Merge(startEnv...); err != nil {
		return nil, nil, errors.WithMessage(err, "failed to create the start env store")
	}

	endEnvStore := newEnvStore()
	if _, err := endEnvStore.Merge(endEnv...); err != nil {
		return nil, nil, errors.WithMessage(err, "failed to create the end env store")
	}

	changed, _, deleted = diffEnvStores(startEnvStore, endEnvStore)
	return
}

func (c *shellEnvCollector) createTempDir() (err error) {
	c.tempDir, err = os.MkdirTemp("", "runme-*")
	if err != nil {
		return errors.WithMessage(err, "failed to create a temporary dir")
	}
	c.owningTempDir = true
	return nil
}

func (c *shellEnvCollector) removeTempDir() error {
	if !c.owningTempDir {
		return nil
	}
	return errors.WithMessage(os.RemoveAll(c.tempDir), "failed to remove the temporary dir")
}

func (c *shellEnvCollector) readEnvFromFile(name string) (result []string, _ error) {
	f, err := os.Open(filepath.Join(c.tempDir, name))
	if err != nil {
		return nil, errors.WithMessagef(err, "failed to open the env file %q", name)
	}
	defer func() { _ = f.Close() }()

	fileInfo, err := f.Stat()
	if err != nil {
		return nil, errors.WithMessagef(err, "failed to get the file info of the env file %q", name)
	}

	scannerBufferSizeInBytes := fileInfo.Size()
	if scannerBufferSizeInBytes > maxScannerBufferSizeInBytes {
		return nil, errors.Errorf("the env file %q is too big: %d bytes", name, scannerBufferSizeInBytes)
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4096), int(scannerBufferSizeInBytes)) // 4096 is taken from bufio as the initial buffer size
	scanner.Split(splitNull)

	for scanner.Scan() {
		result = append(result, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, errors.WithMessagef(err, "failed to scan the env file %q", name)
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

func setEnvDumpTrapOnExit(w io.Writer, cmd string) (int, error) {
	bw := bulkWriter{Writer: w}
	bw.Write(
		[]byte(
			fmt.Sprintf(
				"__cleanup() {\nrv=$?\n%s\nexit $rv\n}\n",
				cmd,
			),
		),
	)
	bw.Write([]byte("trap -- \"__cleanup\" EXIT\n"))
	return bw.Done()
}

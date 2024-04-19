package command

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

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

	scanner := bufio.NewScanner(f)
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

func setEnvDumpTrapOnExit(
	w io.Writer,
	cmd string,
) (_ int, err error) {
	bw := bulkWriter{Writer: w}

	_, _ = bw.Write(
		[]byte(
			fmt.Sprintf(
				"__cleanup() {\nrv=$?\n%s\nexit $rv\n}\n",
				cmd,
			),
		),
	)
	_, _ = bw.Write([]byte("trap -- \"__cleanup\" EXIT\n"))

	return bw.n, bw.err
}

type bulkWriter struct {
	io.Writer
	n   int
	err error
}

func (w *bulkWriter) Write(d []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	n, err := w.Writer.Write(d)
	w.n += n
	w.err = errors.WithStack(err)
	return n, err
}

package command

import (
	"bytes"
	"context"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type fileCommand struct {
	internalCommand

	scriptFile *os.File
	tempDir    string

	logger *zap.Logger
}

func (c *fileCommand) Start(ctx context.Context) error {
	if err := c.createTempDir(); err != nil {
		return err
	}
	if err := c.createScriptFile(); err != nil {
		return err
	}

	buf := new(bytes.Buffer)
	bw := bulkWriter{Writer: buf}
	cfg := c.ProgramConfig()

	// Write the script from the commands or the script.
	if commands := cfg.GetCommands(); commands != nil {
		for _, cmd := range commands.Items {
			bw.WriteString(cmd)
			bw.WriteByte('\n')
		}
	} else if script := cfg.GetScript(); script != "" {
		bw.WriteString(script)
	}

	if err := c.writeScript(buf.String()); err != nil {
		return err
	}

	cfg.Directory = c.tempDir
	// TODO(adamb): it's not always true that the script-based program
	// takes the filename as a last argument.
	cfg.Arguments = append(cfg.Arguments, filepath.Base(c.scriptFile.Name()))

	return c.internalCommand.Start(ctx)
}

func (c *fileCommand) Wait() (err error) {
	defer func() {
		rErr := c.removeTempDir()
		if err == nil {
			err = rErr
		}
	}()
	err = c.internalCommand.Wait()
	return
}

func (c *fileCommand) createTempDir() (err error) {
	c.tempDir, err = os.MkdirTemp("", "runme-*")
	err = errors.WithMessage(err, "failed to create a temporary dir")
	return
}

func (c *fileCommand) createScriptFile() (err error) {
	c.scriptFile, err = os.CreateTemp(c.tempDir, "runme-script-*")
	err = errors.WithMessage(err, "failed to create a temporary file for script execution")
	return
}

func (c *fileCommand) removeTempDir() error {
	if c.tempDir == "" {
		return nil
	}
	c.logger.Info("cleaning up the temporary dir")
	return errors.WithMessage(os.RemoveAll(c.tempDir), "failed to remove the temporary dir")
}

func (c *fileCommand) writeScript(script string) error {
	if _, err := c.scriptFile.Write([]byte(script)); err != nil {
		return errors.WithMessage(err, "failed to write the script to the temporary file")
	}
	return errors.WithMessage(c.scriptFile.Close(), "failed to close the temporary file")
}

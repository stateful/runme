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

	logger *zap.Logger

	scriptFile *os.File
	tempDir    string
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
	pattern := "runme-script-*"
	if ext := c.scriptFileExt(); ext != "" {
		pattern += "." + ext
	}
	c.scriptFile, err = os.CreateTemp(c.tempDir, pattern)
	err = errors.WithMessage(err, "failed to create a temporary file for script execution")
	return
}

func (c *fileCommand) scriptFileExt() string {
	cfg := c.ProgramConfig()
	if ext := cfg.GetFileExtension(); ext != "" {
		return ext
	}
	return inferFileExtension(cfg.GetLanguageId())
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

var fileExtensionByLanguageID = map[string]string{
	"js":              "js",
	"javascript":      "js",
	"jsx":             "jsx",
	"javascriptreact": "jsx",
	"tsx":             "tsx",
	"typescriptreact": "tsx",
	"typescript":      "ts",
	"ts":              "ts",
	"sh":              "sh",
	"bash":            "sh",
	"ksh":             "sh",
	"zsh":             "sh",
	"fish":            "sh",
	"powershell":      "ps1",
	"cmd":             "bat",
	"dos":             "bat",
	"py":              "py",
	"python":          "py",
	"ruby":            "rb",
	"rb":              "rb",
}

func inferFileExtension(languageID string) string {
	return fileExtensionByLanguageID[languageID]
}

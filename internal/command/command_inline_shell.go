package command

import (
	"bytes"
	"context"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type inlineShellCommand struct {
	internalCommand

	envCollector *shellEnvCollector
}

var _ Command = (*inlineShellCommand)(nil)

func newInlineShell(internal internalCommand) *inlineShellCommand {
	return &inlineShellCommand{
		internalCommand: internal,
	}
}

func (c *inlineShellCommand) getPty() *os.File {
	virtualCmd, ok := c.internalCommand.(*virtualCommand)
	if !ok {
		return nil
	}
	return virtualCmd.pty
}

func (c *inlineShellCommand) Start(ctx context.Context) error {
	script, err := c.build()
	if err != nil {
		return err
	}

	cfg := c.ProgramConfig()
	cfg.Arguments = append(cfg.Arguments, "-c", script)

	return c.internalCommand.Start(ctx)
}

func (c *inlineShellCommand) Wait() error {
	err := c.internalCommand.Wait()

	if c.envCollector != nil {
		if cErr := c.collectEnv(); cErr != nil {
			c.Logger().Info("failed to collect the environment", zap.Error(cErr))
			if err == nil {
				err = cErr
			}
		}
	}

	return err
}

func (c *inlineShellCommand) build() (string, error) {
	buf := bytes.NewBuffer(nil)
	bw := bulkWriter{Writer: buf}
	cfg := c.ProgramConfig()

	if options := shellOptionsFromProgram(cfg.ProgramName); options != "" {
		bw.WriteString(options)
		bw.WriteString("\n\n")
	}

	// If the session is provided, we need to collect the environment before and after the script execution.
	// Here, we dump env before the script execution and use trap on EXIT to collect the env after the script execution.
	if c.Session() != nil {
		c.envCollector = &shellEnvCollector{buf: buf}
		if err := c.envCollector.Init(); err != nil {
			return "", err
		}
	}

	// Write the script from the commands or the script.
	if commands := cfg.GetCommands(); commands != nil {
		for _, cmd := range commands.Items {
			bw.WriteString(cmd)
			bw.WriteByte('\n')
		}
	} else if script := cfg.GetScript(); script != "" {
		bw.WriteString(script)
	}

	return buf.String(), nil
}

func (c *inlineShellCommand) collectEnv() error {
	sess := c.Session()

	if sess == nil || c.envCollector == nil {
		return nil
	}

	changed, deleted, err := c.envCollector.Collect()
	if err != nil {
		return err
	}
	if err := sess.SetEnv(changed...); err != nil {
		return errors.WithMessage(err, "failed to set the new or updated env")
	}
	sess.DeleteEnv(deleted...)

	return nil
}

func shellOptionsFromProgram(programPath string) (res string) {
	base := filepath.Base(programPath)
	shell := base[:len(base)-len(filepath.Ext(base))]

	// TODO(mxs): powershell and DOS are missing
	switch shell {
	case "zsh", "ksh", "bash":
		res += "set -e -o pipefail"
	case "sh":
		res += "set -e"
	}

	return
}

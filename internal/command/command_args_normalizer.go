package command

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	runnerv2alpha1 "github.com/stateful/runme/v3/internal/gen/proto/go/runme/runner/v2alpha1"
)

const (
	envStartFileName = ".env_start"
	envEndFileName   = ".env_end"
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

type argsNormalizer struct {
	envCollector *shellEnvCollector
	logger       *zap.Logger
	session      *Session
	scriptFile   *os.File
	tempDir      string
}

func newArgsNormalizer(session *Session, logger *zap.Logger) configNormalizer {
	obj := &argsNormalizer{
		session: session,
		logger:  logger,
	}
	return obj.Normalize
}

func (n *argsNormalizer) Normalize(cfg *Config) (func() error, error) {
	args := append([]string{}, cfg.Arguments...)

	switch cfg.Mode {
	case *runnerv2alpha1.CommandMode_COMMAND_MODE_UNSPECIFIED.Enum():
		panic("invariant: mode unspecified")
	case *runnerv2alpha1.CommandMode_COMMAND_MODE_INLINE.Enum():
		var buf strings.Builder

		if isShellLanguage(cfg.LanguageId) {
			if err := n.inlineShell(cfg, &buf); err != nil {
				return nil, err
			}
		} else {
			// Write the script from the commands or the script.
			if commands := cfg.GetCommands(); commands != nil {
				for _, cmd := range commands.Items {
					_, _ = buf.WriteString(cmd)
					_, _ = buf.WriteRune('\n')
				}
			} else if script := cfg.GetScript(); script != "" {
				_, _ = buf.WriteString(script)
			}
		}

		// TODO(adamb): "-c" is not supported for all inline programs.
		if val := buf.String(); val != "" {
			args = append(args, "-c", val)
		}

	case *runnerv2alpha1.CommandMode_COMMAND_MODE_FILE.Enum():
		if err := n.createTempDir(); err != nil {
			return nil, err
		}

		if err := n.createScriptFile(); err != nil {
			return nil, err
		}

		if err := n.writeScript([]byte(cfg.GetScript())); err != nil {
			return nil, err
		}

		// TODO(adamb): it's not always true that the script-based program
		// takes the filename as a last argument.
		args = append(args, n.scriptFile.Name())

	case *runnerv2alpha1.CommandMode_COMMAND_MODE_TERMINAL.Enum():
		// noop
	}

	cfg.Arguments = args

	return n.cleanup, nil
}

func (n *argsNormalizer) inlineShell(cfg *Config, buf *strings.Builder) error {
	if options := shellOptionsFromProgram(cfg.ProgramName); options != "" {
		_, _ = buf.WriteString(options)
		_, _ = buf.WriteString("\n\n")
	}

	// If the session is provided, the env should be collected.
	if n.session != nil {
		n.envCollector = &shellEnvCollector{
			buf: buf,
		}
		if err := n.envCollector.Init(); err != nil {
			return err
		}
	}

	// Write the script from the commands or the script.
	if commands := cfg.GetCommands(); commands != nil {
		for _, cmd := range commands.Items {
			_, _ = buf.WriteString(cmd)
			_, _ = buf.WriteRune('\n')
		}
	} else if script := cfg.GetScript(); script != "" {
		_, _ = buf.WriteString(script)
	}

	return nil
}

func (n *argsNormalizer) createTempDir() (err error) {
	n.tempDir, err = os.MkdirTemp("", "runme-*")
	err = errors.WithMessage(err, "failed to create a temporary dir")
	return
}

func (n *argsNormalizer) removeTempDir() error {
	if n.tempDir == "" {
		return nil
	}

	n.logger.Info("cleaning up the temporary dir")

	if err := os.RemoveAll(n.tempDir); err != nil {
		return errors.WithMessage(err, "failed to remove the temporary dir")
	}

	return nil
}

func (n *argsNormalizer) cleanup() (result error) {
	if err := n.collectEnv(); err != nil {
		result = multierr.Append(result, err)
	}
	if err := n.removeTempDir(); err != nil {
		result = multierr.Append(result, err)
	}
	return
}

func (n *argsNormalizer) collectEnv() error {
	if n.session == nil || n.envCollector == nil {
		return nil
	}

	changed, deleted, err := n.envCollector.Collect()
	if err != nil {
		return err
	}

	if err := n.session.SetEnv(changed...); err != nil {
		return errors.WithMessage(err, "failed to set the new or updated env")
	}

	n.session.DeleteEnv(deleted...)

	return nil
}

func (n *argsNormalizer) createScriptFile() (err error) {
	n.scriptFile, err = os.CreateTemp(n.tempDir, "runme-script-*")
	err = errors.WithMessage(err, "failed to create a temporary file for script execution")
	return
}

func (n *argsNormalizer) writeScript(script []byte) error {
	if _, err := n.scriptFile.Write(script); err != nil {
		return errors.WithMessage(err, "failed to write the script to the temporary file")
	}
	return errors.WithMessage(n.scriptFile.Close(), "failed to close the temporary file")
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

package command

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	runnerv2alpha1 "github.com/stateful/runme/internal/gen/proto/go/runme/runner/v2alpha1"
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
	session *Session
	logger  *zap.Logger

	tempDir          string
	isEnvCollectable bool
	scriptFile       *os.File
}

func newArgsNormalizer(session *Session, logger *zap.Logger) configNormalizer {
	return (&argsNormalizer{session: session, logger: logger}).Normalize
}

func (n *argsNormalizer) Normalize(cfg *Config) (*Config, func() error, error) {
	args := append([]string{}, cfg.Arguments...)

	switch cfg.Mode {
	case *runnerv2alpha1.CommandMode_COMMAND_MODE_UNSPECIFIED.Enum():
		panic("invariant: mode unspecified")
	case *runnerv2alpha1.CommandMode_COMMAND_MODE_INLINE.Enum():
		var buf strings.Builder

		if isShellLanguage(filepath.Base(cfg.ProgramName)) {
			if err := n.inlineShell(cfg, &buf); err != nil {
				return nil, nil, err
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
			return nil, nil, err
		}

		if err := n.createScriptFile(); err != nil {
			return nil, nil, err
		}

		if err := n.writeScript([]byte(cfg.GetScript())); err != nil {
			return nil, nil, err
		}

		// TODO(adamb): it's not always true that the script-based program
		// takes the filename as a last argument.
		args = append(args, n.scriptFile.Name())
	}

	result := proto.Clone(cfg).(*Config)
	result.Arguments = args
	return result, n.cleanup, nil
}

func (n *argsNormalizer) inlineShell(cfg *Config, buf *strings.Builder) error {
	if options := shellOptionsFromProgram(cfg.ProgramName); options != "" {
		_, _ = buf.WriteString(options)
		_, _ = buf.WriteString("\n\n")
	}

	// If the session is provided, we need to collect the environment before and after the script execution.
	// Here, we dump env before the script execution and use trap on EXIT to collect the env after the script execution.
	if n.session != nil {
		if err := n.createTempDir(); err != nil {
			return err
		}

		_, _ = buf.WriteString(fmt.Sprintf("%s > %s\n", EnvDumpCommand, filepath.Join(n.tempDir, envStartFileName)))
		_, _ = buf.WriteString(fmt.Sprintf("__cleanup() {\nrv=$?\n%s > %s\nexit $rv\n}\n", EnvDumpCommand, filepath.Join(n.tempDir, envEndFileName)))
		_, _ = buf.WriteString("trap -- \"__cleanup\" EXIT\n")

		n.isEnvCollectable = true
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

func (n *argsNormalizer) cleanup() (result error) {
	if err := n.collectEnv(); err != nil {
		result = multierr.Append(result, err)
	}
	if err := n.removeTempDir(); err != nil {
		result = multierr.Append(result, err)
	}
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

func (n *argsNormalizer) collectEnv() error {
	if n.session == nil || !n.isEnvCollectable {
		return nil
	}

	n.logger.Info("collecting env")

	startEnv, err := n.readEnvFromFile(envStartFileName)
	if err != nil {
		return err
	}

	endEnv, err := n.readEnvFromFile(envEndFileName)
	if err != nil {
		return err
	}

	// Below, we diff the env collected before and after the script execution.
	// Then, update the session with the new or updated env and delete the deleted env.

	startEnvStore := newEnvStore()
	if _, err := startEnvStore.Merge(startEnv...); err != nil {
		return errors.WithMessage(err, "failed to create the start env store")
	}

	endEnvStore := newEnvStore()
	if _, err := endEnvStore.Merge(endEnv...); err != nil {
		return errors.WithMessage(err, "failed to create the end env store")
	}

	newOrUpdated, _, deleted := diffEnvStores(startEnvStore, endEnvStore)

	if err := n.session.SetEnv(newOrUpdated...); err != nil {
		return errors.WithMessage(err, "failed to set the new or updated env")
	}

	n.session.DeleteEnv(deleted...)

	return nil
}

func (n *argsNormalizer) createTempDir() (err error) {
	n.tempDir, err = os.MkdirTemp("", "runme-*")
	err = errors.WithMessage(err, "failed to create  atemporery dir")
	return
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

func (n *argsNormalizer) readEnvFromFile(name string) (result []string, _ error) {
	f, err := os.Open(filepath.Join(n.tempDir, name))
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

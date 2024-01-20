package command

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
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

func (n *argsNormalizer) Normalize(cfg *Config) (*Config, error) {
	args := append([]string{}, cfg.Arguments...)

	switch cfg.Mode {
	case *runnerv2alpha1.CommandMode_COMMAND_MODE_UNSPECIFIED.Enum():
		panic("invariant: mode unspecified")
	case *runnerv2alpha1.CommandMode_COMMAND_MODE_INLINE.Enum():
		var buf strings.Builder

		if isShellLanguage(filepath.Base(cfg.ProgramName)) {
			if options := shellOptionsFromProgram(cfg.ProgramName); options != "" {
				_, _ = buf.WriteString(options)
				_, _ = buf.WriteString("\n\n")
			}

			if n.session != nil {
				if err := n.createTempDir(); err != nil {
					return nil, err
				}
				_, _ = buf.WriteString(fmt.Sprintf("%s > %s\n", EnvDumpCommand, filepath.Join(n.tempDir, envStartFileName)))
			}
		}

		if commands := cfg.GetCommands(); commands != nil {
			for _, cmd := range commands.Items {
				_, _ = buf.WriteString(cmd)
				_, _ = buf.WriteRune('\n')
			}
		} else if script := cfg.GetScript(); script != "" {
			_, _ = buf.WriteString(script)
		}

		if isShellLanguage(filepath.Base(cfg.ProgramName)) {
			if n.session != nil {
				_, _ = buf.WriteString(fmt.Sprintf("%s > %s\n", EnvDumpCommand, filepath.Join(n.tempDir, envEndFileName)))

				n.isEnvCollectable = true
			}
		}

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
	}

	result := proto.Clone(cfg).(*Config)
	result.Arguments = args
	return result, nil
}

func (n *argsNormalizer) Cleanup() {
	if n.tempDir == "" {
		return
	}

	n.logger.Info("cleaning up the temporary dir")

	if err := os.RemoveAll(n.tempDir); err != nil {
		n.logger.Info("failed to remove temporary dir", zap.Error(err))
	}
}

func (n *argsNormalizer) CollectEnv() {
	if n.session == nil || !n.isEnvCollectable {
		return
	}

	n.logger.Info("collecting env")

	startEnv, err := n.readEnvFromFile(envStartFileName)
	if err != nil {
		n.logger.Info("failed to read the start env file", zap.Error(err))
		return
	}

	endEnv, err := n.readEnvFromFile(envEndFileName)
	if err != nil {
		n.logger.Info("failed to read the end env file", zap.Error(err))
		return
	}

	startEnvStore := newEnvStore()
	if _, err := startEnvStore.Merge(startEnv...); err != nil {
		n.logger.Info("failed to create the start env store", zap.Error(err))
		return
	}

	endEnvStore := newEnvStore()
	if _, err := endEnvStore.Merge(endEnv...); err != nil {
		n.logger.Info("failed to create the end env store", zap.Error(err))
		return
	}

	newOrUpdated, _, deleted := diffEnvStores(startEnvStore, endEnvStore)

	if err := n.session.SetEnv(newOrUpdated...); err != nil {
		n.logger.Info("failed to set the new or updated env", zap.Error(err))
		return
	}

	n.session.DeleteEnv(deleted...)
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
		return nil, errors.WithStack(err)
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	scanner.Split(splitNull)

	for scanner.Scan() {
		result = append(result, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, errors.WithStack(err)
	}

	return result, errors.WithStack(scanner.Err())
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

type envSource func() []string

type envNormalizer struct {
	sources []envSource
}

func (n *envNormalizer) Normalize(cfg *Config) (*Config, error) {
	result := proto.Clone(cfg).(*Config)

	env := os.Environ()
	env = append(env, cfg.Env...)

	for _, source := range n.sources {
		env = append(env, source()...)
	}

	result.Env = env

	return result, nil
}

package command

import (
	"strings"

	runnerv2alpha1 "github.com/stateful/runme/v3/internal/gen/proto/go/runme/runner/v2alpha1"
	"google.golang.org/protobuf/proto"
)

// Config contains a serializable configuration for a command.
// It's agnostic to the runtime or particular execution settings.
type Config = runnerv2alpha1.ProgramConfig

// configNormalizer is a function that normalizes [Config].
// It will modify the [Config] in place and return a cleanup function
// that should be executed when the command is finished.
type configNormalizer func(*Config) (func() error, error)

func normalizeConfig(cfg *Config, normalizers ...configNormalizer) (_ *Config, cleanups []func() error, err error) {
	cfg = proto.Clone(cfg).(*Config)

	for _, normalizer := range normalizers {
		var cleanup func() error

		cleanup, err = normalizer(cfg)
		if err != nil {
			return nil, nil, err
		}

		if cleanup != nil {
			cleanups = append(cleanups, cleanup)
		}
	}
	return cfg, cleanups, nil
}

// redactConfig returns a new Config instance and copies only fields considered safe.
// Useful for logging.
func redactConfig(cfg *Config) *Config {
	return &Config{
		ProgramName: cfg.ProgramName,
		Arguments:   cfg.Arguments,
		Directory:   cfg.Directory,
		Source:      cfg.Source,
		Interactive: cfg.Interactive,
		Mode:        cfg.Mode,
	}
}

// TODO(adamb): this function is used for two quite different inputs: program name and language ID.
func isShellLanguage(languageID string) bool {
	switch strings.ToLower(languageID) {
	// shellscripts
	// TODO(adamb): breaking change: shellscript was removed to indicate
	// that it should be executed as a file. Consider adding it back and
	// using attributes to decide how a code block should be executed.
	case "sh", "bash", "zsh", "ksh", "shell", "shellscript":
		return true

	// dos
	case "bat", "cmd":
		return true

	// powershell
	case "powershell", "pwsh":
		return true

	// fish
	case "fish":
		return true

	default:
		return false
	}
}

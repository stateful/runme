package command

import (
	"path/filepath"

	runnerv2alpha1 "github.com/stateful/runme/v3/pkg/gen/proto/go/runme/runner/v2alpha1"
)

func modeNormalizer(cfg *Config) (func() error, error) {
	if cfg.Mode != runnerv2alpha1.CommandMode_COMMAND_MODE_UNSPECIFIED {
		return nil, nil
	}

	// If the mode is not specified, we check the program name to determine the mode.
	// This is mostly for backward compatibility.
	if isShellLanguage(filepath.Base(cfg.ProgramName)) {
		cfg.Mode = runnerv2alpha1.CommandMode_COMMAND_MODE_INLINE
	} else {
		cfg.Mode = runnerv2alpha1.CommandMode_COMMAND_MODE_FILE
	}

	return nil, nil
}

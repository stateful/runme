package command

import (
	"path/filepath"

	"google.golang.org/protobuf/proto"

	runnerv2alpha1 "github.com/stateful/runme/v3/internal/gen/proto/go/runme/runner/v2alpha1"
)

func modeNormalizer(cfg *Config) (*Config, func() error, error) {
	if cfg.Mode != runnerv2alpha1.CommandMode_COMMAND_MODE_UNSPECIFIED {
		return cfg, nil, nil
	}

	result := proto.Clone(cfg).(*Config)

	// If the mode is not specified, we check the program name to determine the mode.
	// This is mostly for backward compatibility.
	if isShellLanguage(filepath.Base(result.ProgramName)) {
		result.Mode = runnerv2alpha1.CommandMode_COMMAND_MODE_INLINE
	} else {
		result.Mode = runnerv2alpha1.CommandMode_COMMAND_MODE_FILE
	}

	return result, nil, nil
}

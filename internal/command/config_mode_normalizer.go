package command

import (
	"path/filepath"

	"google.golang.org/protobuf/proto"

	runnerv2alpha1 "github.com/stateful/runme/internal/gen/proto/go/runme/runner/v2alpha1"
)

func modeNormalizer(cfg *Config) (*Config, configNormalizerCleanupFunc, error) {
	if cfg.Mode != runnerv2alpha1.CommandMode_COMMAND_MODE_UNSPECIFIED {
		return cfg, nil, nil
	}

	result := proto.Clone(cfg).(*Config)

	if isShellLanguage(filepath.Base(result.ProgramName)) {
		result.Mode = runnerv2alpha1.CommandMode_COMMAND_MODE_INLINE
	} else {
		result.Mode = runnerv2alpha1.CommandMode_COMMAND_MODE_FILE
	}

	return result, nil, nil
}

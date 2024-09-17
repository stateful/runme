package command

import (
	"path/filepath"
	"strings"

	runnerv2 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/runner/v2"
)

// ProgramConfig contains a serializable configuration for a command.
// It's agnostic to the runtime or particular execution settings.
type ProgramConfig = runnerv2.ProgramConfig

// redactConfig returns a new [ProgramConfig] instance and copies only fields considered safe.
// Useful for logging.
func redactConfig(cfg *ProgramConfig) *ProgramConfig {
	return &ProgramConfig{
		ProgramName: cfg.ProgramName,
		Arguments:   cfg.Arguments,
		Directory:   cfg.Directory,
		Source:      cfg.Source,
		Interactive: cfg.Interactive,
		Mode:        cfg.Mode,
	}
}

func isShell(cfg *ProgramConfig) bool {
	return IsShellProgram(filepath.Base(cfg.ProgramName)) || IsShellLanguage(cfg.LanguageId)
}

func IsShellProgram(programName string) bool {
	switch strings.ToLower(programName) {
	case "sh", "bash", "zsh", "ksh", "shell":
		return true
	case "cmd", "powershell", "pwsh", "fish":
		return true
	default:
		return false
	}
}

// TODO(adamb): this function is used for two quite different inputs: program name and language ID.
func IsShellLanguage(languageID string) bool {
	switch strings.ToLower(languageID) {
	// shellscripts
	// TODO(adamb): breaking change: shellscript was removed to indicate
	// that it should be executed as a file. Consider adding it back and
	// using attributes to decide how a code block should be executed.
	case "sh", "bash", "zsh", "ksh", "shell":
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

package command

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"google.golang.org/protobuf/proto"

	runnerv2alpha1 "github.com/stateful/runme/internal/gen/proto/go/runme/runner/v2alpha1"
)

type ErrUnsupportedLanguage struct {
	langID string
}

func (e ErrUnsupportedLanguage) Error() string {
	return fmt.Sprintf("unsupported language %s", e.langID)
}

type ErrInterpretersNotFound struct {
	interpreters []string
}

func (e ErrInterpretersNotFound) Error() string {
	return fmt.Sprintf("unable to loop up any of interpreters %q", e.interpreters)
}

// Config contains a serializable configuration for a command.
// It's agnostic to the runtime or particular execution settings.
type Config = runnerv2alpha1.ProgramConfig

func NormalizeConfigForDryRun(cfg *Config) (*Config, error) {
	return normalizeConfig(cfg, &argsNormalizer{})
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

func normalizeConfig(cfg *Config, extra ...configNormalizer) (*Config, error) {
	normalizers := []configNormalizer{
		&pathNormalizer{},
		&modeNormalizer{},
	}

	normalizers = append(normalizers, extra...)

	for _, normalizer := range normalizers {
		var err error

		if cfg, err = normalizer.Normalize(cfg); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}

type configNormalizer interface {
	Normalize(*Config) (*Config, error)
}

type pathNormalizer struct{}

func (n *pathNormalizer) Normalize(cfg *Config) (*Config, error) {
	programPath, err := exec.LookPath(cfg.ProgramName)
	if err == nil {
		if programPath == cfg.ProgramName {
			return cfg, nil
		}

		result := proto.Clone(cfg).(*Config)
		result.ProgramName = programPath

		return result, nil
	}

	interpreters := inferInterpreterFromLanguage(cfg.ProgramName)
	if len(interpreters) == 0 {
		return nil, &ErrUnsupportedLanguage{langID: cfg.ProgramName}
	}

	for _, interpreter := range interpreters {
		program, args := parseInterpreter(interpreter)
		if programPath, err := exec.LookPath(program); err == nil {
			result := proto.Clone(cfg).(*Config)
			result.ProgramName = programPath
			result.Arguments = args
			return result, nil
		}
	}

	return nil, &ErrInterpretersNotFound{interpreters: interpreters}
}

type modeNormalizer struct{}

func (n *modeNormalizer) Normalize(cfg *Config) (*Config, error) {
	if cfg.Mode != runnerv2alpha1.CommandMode_COMMAND_MODE_UNSPECIFIED {
		return cfg, nil
	}

	result := proto.Clone(cfg).(*Config)

	if isShellLanguage(filepath.Base(result.ProgramName)) {
		result.Mode = runnerv2alpha1.CommandMode_COMMAND_MODE_INLINE
	} else {
		result.Mode = runnerv2alpha1.CommandMode_COMMAND_MODE_FILE
	}

	return result, nil
}

func prepareScriptFromLines(programPath string, lines []string) string {
	var buf strings.Builder

	for _, cmd := range lines {
		_, _ = buf.WriteString(cmd)
		_, _ = buf.WriteRune('\n')
	}

	return buf.String()
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

func isShellLanguage(languageID string) bool {
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

// parseInterpreter handles cases when the interpreter is, for instance, "deno run".
// Only the first word is a program name and the rest is arguments.
func parseInterpreter(interpreter string) (program string, args []string) {
	parts := strings.SplitN(interpreter, " ", 2)

	if len(parts) > 0 {
		program = parts[0]
	}

	if len(parts) > 1 {
		args = strings.Split(parts[1], " ")
	}

	return
}

var interpreterByLanguageID = map[string][]string{
	"js":              {"node"},
	"javascript":      {"node"},
	"jsx":             {"node"},
	"javascriptreact": {"node"},

	"ts":              {"ts-node", "deno run", "bun run"},
	"typescript":      {"ts-node", "deno run", "bun run"},
	"tsx":             {"ts-node", "deno run", "bun run"},
	"typescriptreact": {"ts-node", "deno run", "bun run"},

	"sh":          {"bash", "sh"},
	"bash":        {"bash", "sh"},
	"ksh":         {"ksh"},
	"zsh":         {"zsh"},
	"fish":        {"fish"},
	"powershell":  {"powershell"},
	"cmd":         {"cmd"},
	"dos":         {"cmd"},
	"shellscript": {"bash", "sh"},

	"lua":    {"lua"},
	"perl":   {"perl"},
	"php":    {"php"},
	"python": {"python3", "python"},
	"py":     {"python3", "python"},
	"ruby":   {"ruby"},
	"rb":     {"ruby"},
}

func inferInterpreterFromLanguage(langID string) []string {
	return interpreterByLanguageID[langID]
}

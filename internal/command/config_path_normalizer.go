package command

import (
	"fmt"
	"os/exec"
	"strings"

	"google.golang.org/protobuf/proto"
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
	return fmt.Sprintf("unable to look up any of interpreters %q", e.interpreters)
}

func pathNormalizer(cfg *Config) (*Config, configNormalizerCleanupFunc, error) {
	programPath, err := exec.LookPath(cfg.ProgramName)
	if err == nil {
		if programPath == cfg.ProgramName {
			return cfg, nil, nil
		}

		result := proto.Clone(cfg).(*Config)
		result.ProgramName = programPath

		return result, nil, nil
	}

	interpreters := inferInterpreterFromLanguage(cfg.ProgramName)
	if len(interpreters) == 0 {
		return nil, nil, &ErrUnsupportedLanguage{langID: cfg.ProgramName}
	}

	for _, interpreter := range interpreters {
		program, args := parseInterpreter(interpreter)
		if programPath, err := exec.LookPath(program); err == nil {
			result := proto.Clone(cfg).(*Config)
			result.ProgramName = programPath
			result.Arguments = args
			return result, nil, nil
		}
	}

	return nil, nil, &ErrInterpretersNotFound{interpreters: interpreters}
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

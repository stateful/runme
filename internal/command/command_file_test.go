//go:build !windows

package command

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stateful/runme/v3/internal/session"
	runnerv2 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/runner/v2"
)

func TestFileCommand(t *testing.T) {
	t.Parallel()

	t.Run("Shell", func(t *testing.T) {
		t.Parallel()

		cfg := &ProgramConfig{
			ProgramName: "bash",
			Source: &runnerv2.ProgramConfig_Commands{
				Commands: &runnerv2.ProgramConfig_CommandList{
					Items: []string{"echo -n test"},
				},
			},
			Mode: runnerv2.CommandMode_COMMAND_MODE_FILE,
		}

		testExecuteCommand(t, cfg, nil, "test", "")
	})

	t.Run("Shellscript", func(t *testing.T) {
		t.Parallel()

		cfg := &ProgramConfig{
			ProgramName: "",
			LanguageId:  "shellscript",
			Source: &runnerv2.ProgramConfig_Commands{
				Commands: &runnerv2.ProgramConfig_CommandList{
					Items: []string{`echo -n "execute shellscript as shell script"`},
				},
			},
			Mode: runnerv2.CommandMode_COMMAND_MODE_FILE,
		}

		testExecuteCommand(t, cfg, nil, "execute shellscript as shell script", "")
	})

	t.Run("Python", func(t *testing.T) {
		t.Parallel()

		cfg := &ProgramConfig{
			ProgramName: "python3",
			Source: &runnerv2.ProgramConfig_Script{
				Script: "print('test')",
			},
			Mode: runnerv2.CommandMode_COMMAND_MODE_FILE,
		}

		testExecuteCommand(t, cfg, nil, "test\n", "")
	})

	// TypeScript runner requires the file extension to be .ts.
	t.Run("TypeScript", func(t *testing.T) {
		t.Parallel()

		cfg := &ProgramConfig{
			LanguageId: "ts",
			Source: &runnerv2.ProgramConfig_Script{
				Script: `function print(message: string): void {
	console.log(message)
}
print("important message")
`,
			},
			Mode: runnerv2.CommandMode_COMMAND_MODE_FILE,
		}

		testExecuteCommand(t, cfg, nil, "important message\n", "")
	})

	t.Run("TypeScriptWithDeno", func(t *testing.T) {
		t.Parallel()

		cfg := &ProgramConfig{
			LanguageId:  "ts",
			ProgramName: "deno run -A",
			Source: &runnerv2.ProgramConfig_Script{
				Script: `function print(message: string): void {
	console.log(message)
}
print("important message")
`,
			},
			Mode: runnerv2.CommandMode_COMMAND_MODE_FILE,
		}

		testExecuteCommand(t, cfg, nil, "important message\n", "")
	})

	t.Run("RustScript", func(t *testing.T) {
		t.Parallel()

		// Rust is like Python. Envs are user-local and need to be sourced.
		cargoEnvCfg := &ProgramConfig{
			ProgramName: "bash",
			Source: &runnerv2.ProgramConfig_Script{
				Script: `source "$HOME/.cargo/env"`,
			},
			Mode: runnerv2.CommandMode_COMMAND_MODE_INLINE,
		}
		sess, err := session.New(session.WithSeedEnv(os.Environ()))
		require.NoError(t, err)

		testExecuteCommandWithSession(t, cargoEnvCfg, sess, nil, "", "")

		cfg := &ProgramConfig{
			LanguageId: "rust",
			Source: &runnerv2.ProgramConfig_Script{
				Script: `fn main() { println!("Running rust code") }`,
			},
			Mode: runnerv2.CommandMode_COMMAND_MODE_FILE,
		}

		testExecuteCommandWithSession(t, cfg, sess, nil, "Running rust code\n", "")
	})
}

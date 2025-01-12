package beta

import (
	"context"
	"io"
	"os"
	"os/exec"
	"strconv"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/stateful/runme/v3/internal/command"
	"github.com/stateful/runme/v3/internal/config/autoconfig"
	runnerv2 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/runner/v2"
)

func sessionCmd(*commonFlags) *cobra.Command {
	cmd := cobra.Command{
		Use:   "session",
		Short: "Start shell within a session.",
		Long: `Start shell within a session.

All exported variables during the session will be available to the subsequent commands.
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return autoconfig.Invoke(
				func(
					cmdFactory command.Factory,
					logger *zap.Logger,
				) error {
					defer logger.Sync()

					envs, err := executeDefaultShellProgram(
						cmd.Context(),
						cmdFactory,
						cmd.InOrStdin(),
						cmd.OutOrStdout(),
						cmd.ErrOrStderr(),
						nil,
					)
					if err != nil {
						return err
					}

					// TODO(adamb): currently, the collected env are printed out,
					// but they could be put in a session.
					if _, err := cmd.ErrOrStderr().Write([]byte("Collected env during the session:\n")); err != nil {
						return errors.WithStack(err)
					}

					for _, env := range envs {
						_, err := cmd.OutOrStdout().Write([]byte(env + "\n"))
						if err != nil {
							return errors.WithStack(err)
						}
					}

					return nil
				},
			)
		},
	}

	cmd.AddCommand(sessionSetupCmd())

	return &cmd
}

func executeDefaultShellProgram(
	ctx context.Context,
	commandFactory command.Factory,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
	additionalEnv []string,
) ([]string, error) {
	envCollector, err := command.NewEnvCollectorFactory().Build()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	cfg := &command.ProgramConfig{
		ProgramName: defaultShell(),
		Mode:        runnerv2.CommandMode_COMMAND_MODE_CLI,
		Env: append(
			[]string{command.CreateEnv(command.EnvNameTerminalSessionEnabled, "true")},
			append(envCollector.ExtraEnv(), additionalEnv...)...,
		),
	}
	options := command.CommandOptions{
		NoShell: true,
		Stdin:   stdin,
		Stdout:  stdout,
		Stderr:  stderr,
	}
	program, err := commandFactory.Build(cfg, options)
	if err != nil {
		return nil, err
	}

	err = program.Start(ctx)
	if err != nil {
		return nil, err
	}

	err = program.Wait(ctx)
	if err != nil {
		return nil, err
	}

	changed, _, err := envCollector.Diff()
	return changed, err
}

func defaultShell() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell, _ = exec.LookPath("bash")
	}
	if shell == "" {
		shell = "/bin/sh"
	}
	return shell
}

func sessionSetupCmd() *cobra.Command {
	var debug bool

	cmd := cobra.Command{
		Use:    "setup",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return autoconfig.Invoke(
				func(
					cmdFactory command.Factory,
					logger *zap.Logger,
				) error {
					defer logger.Sync()

					out := cmd.OutOrStdout()

					if err := requireEnvs(
						command.EnvNameTerminalSessionEnabled,
						command.EnvNameTerminalSessionPrePath,
						command.EnvNameTerminalSessionPostPath,
					); err != nil {
						logger.Info("session setup is skipped because the environment variable is not set", zap.Error(err))
						return writeNoopShellCommand(out)
					}

					sessionSetupEnabled := os.Getenv(command.EnvNameTerminalSessionEnabled)
					if val, err := strconv.ParseBool(sessionSetupEnabled); err != nil || !val {
						logger.Debug("session setup is skipped", zap.Error(err), zap.Bool("value", val))
						return writeNoopShellCommand(out)
					}

					envSetter := command.NewScriptEnvSetter(
						os.Getenv(command.EnvNameTerminalSessionPrePath),
						os.Getenv(command.EnvNameTerminalSessionPostPath),
						debug,
					)
					if err := envSetter.SetOnShell(out); err != nil {
						return err
					}

					if _, err := cmd.ErrOrStderr().Write([]byte("Runme session active. When you're done, execute \"exit\".\n")); err != nil {
						return errors.WithStack(err)
					}

					return nil
				},
			)
		},
	}

	cmd.Flags().BoolVar(&debug, "debug", false, "Enable debug mode.")

	return &cmd
}

func requireEnvs(names ...string) error {
	var err error
	for _, name := range names {
		if os.Getenv(name) == "" {
			err = multierr.Append(err, errors.Errorf("environment variable %q is required", name))
		}
	}
	return err
}

func writeNoopShellCommand(w io.Writer) error {
	_, err := w.Write([]byte(":"))
	return errors.WithStack(err)
}

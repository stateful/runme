package beta

import (
	"os"
	"os/exec"
	"strconv"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
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
			return autoconfig.InvokeForCommand(
				func(
					cmdFactory command.Factory,
					logger *zap.Logger,
				) error {
					defer logger.Sync()

					envCollector, err := command.NewEnvCollectorFactory().Build()
					if err != nil {
						return errors.WithStack(err)
					}

					cfg := &command.ProgramConfig{
						ProgramName: defaultShell(),
						Mode:        runnerv2.CommandMode_COMMAND_MODE_CLI,
						Env:         append([]string{"RUNME_SESSION=1"}, envCollector.ExtraEnv()...),
					}
					options := command.CommandOptions{
						NoShell: true,
						Stdin:   cmd.InOrStdin(),
						Stdout:  cmd.OutOrStdout(),
						Stderr:  cmd.ErrOrStderr(),
					}

					program, err := cmdFactory.Build(cfg, options)
					if err != nil {
						return err
					}

					err = program.Start(cmd.Context())
					if err != nil {
						return err
					}

					err = program.Wait()
					if err != nil {
						return err
					}

					changed, _, err := envCollector.Diff()
					if err != nil {
						return errors.WithStack(err)
					}

					// TODO(adamb): currently, the collected env are printed out,
					// but they could be put in a session.
					if _, err := cmd.ErrOrStderr().Write([]byte("Collected env during the session:\n")); err != nil {
						return errors.WithStack(err)
					}
					for _, env := range changed {
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

func sessionSetupCmd() *cobra.Command {
	var debug bool

	cmd := cobra.Command{
		Use:    "setup",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return autoconfig.InvokeForCommand(
				func(
					cmdFactory command.Factory,
					logger *zap.Logger,
				) error {
					defer logger.Sync()

					if val, err := strconv.ParseBool(os.Getenv(command.EnvCollectorSessionEnvName)); err != nil || !val {
						logger.Debug("session setup is skipped", zap.Error(err), zap.Bool("value", val))
						return nil
					}

					envSetter := command.NewFileBasedEnvSetter(
						os.Getenv(command.EnvCollectorSessionPrePathEnvName),
						os.Getenv(command.EnvCollectorSessionPostPathEnvName),
						debug,
					)

					err := envSetter.SetOnShell(cmd.OutOrStdout())
					return errors.WithStack(err)
				},
			)
		},
	}

	cmd.Flags().BoolVar(&debug, "debug", false, "Enable debug mode.")

	return &cmd
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

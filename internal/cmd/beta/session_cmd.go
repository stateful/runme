package beta

import (
	"bytes"
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

					envCollector, err := command.NewEnvCollectorFactory().UseFifo(false).Build()
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

					runmeCmd, err := cmdFactory.Build(cfg, options)
					if err != nil {
						return err
					}

					err = runmeCmd.Start(cmd.Context())
					if err != nil {
						return err
					}

					err = runmeCmd.Wait()
					if err != nil {
						return err
					}

					changed, _, err := envCollector.Diff()
					if err != nil {
						return errors.WithStack(err)
					}

					for _, env := range changed {
						_, err := cmd.OutOrStdout().Write([]byte(env + "\n"))
						if err != nil {
							return err
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
					)
					buf := new(bytes.Buffer)

					_, _ = buf.WriteString("#!/bin/sh\n")
					_, _ = buf.WriteString("set -euxo pipefail\n")
					_ = envSetter.SetOnShell(buf)
					_, _ = buf.WriteString("set +euxo pipefail\n")

					_, err := cmd.OutOrStdout().Write(buf.Bytes())
					return errors.WithStack(err)
				},
			)
		},
	}

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

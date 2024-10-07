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

					cfg := &command.ProgramConfig{
						ProgramName: defaultShell(),
						Mode:        runnerv2.CommandMode_COMMAND_MODE_CLI,
						Env:         []string{"RUNME_SESSION=1"},
					}
					session := command.NewSession()
					options := getCommandOptions(cmd, session)
					options.NoShell = true

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

					for _, env := range session.GetAllEnv() {
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

					if val, err := strconv.ParseBool(os.Getenv("RUNME_SESSION")); err != nil || !val {
						logger.Debug("session setup is skipped", zap.Error(err), zap.Bool("value", val))
						return nil
					}

					envCollector, err := command.NewEnvCollectorFactory().UseFifo(false).Build()
					if err != nil {
						return errors.WithStack(err)
					}

					var buf bytes.Buffer

					_, _ = buf.WriteString("#!/bin/sh\nset -euxo pipefail\n")

					err = envCollector.SetOnShell(&buf)
					if err != nil {
						return errors.WithStack(err)
					}

					_, _ = buf.WriteString("set +euxo pipefail\n")

					_, err = cmd.OutOrStdout().Write(buf.Bytes())
					return errors.WithStack(err)

					// _, err = cmd.OutOrStdout().Write([]byte("#!/bin/sh\nset -euxo pipefail\n"))
					// if err != nil {
					// 	return err
					// }
					// _, err = cmd.OutOrStdout().Write([]byte("trap -- \"echo exited\" EXIT\n"))
					// if err != nil {
					// 	return err
					// }
					// _, err = cmd.OutOrStdout().Write([]byte("echo 'setup done'\n"))
					// if err != nil {
					// 	return err
					// }
					// _, err = cmd.OutOrStdout().Write([]byte("set +euxo pipefail\n"))
					// return err
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

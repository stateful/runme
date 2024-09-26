package server

import (
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/stateful/runme/v3/internal/command"
	"github.com/stateful/runme/v3/internal/config"
	"github.com/stateful/runme/v3/internal/config/autoconfig"
	"github.com/stateful/runme/v3/internal/server"
	"github.com/stateful/runme/v3/internal/telemetry"
)

func serverStartCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:   "start",
		Short: "Start a server.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return autoconfig.InvokeForCommand(
				func(
					cfg *config.Config,
					cmdFactory command.Factory,
					logger *zap.Logger,
				) error {
					defer logger.Sync()

					serverCfg := &server.Config{
						Address:    cfg.Server.Address,
						CertFile:   *cfg.Server.Tls.CertFile, // guaranteed by autoconfig
						KeyFile:    *cfg.Server.Tls.KeyFile,  // guaranteed by autoconfig
						TLSEnabled: cfg.Server.Tls.Enabled,
					}

					_ = telemetry.ReportUnlessNoTracking(logger)

					logger.Debug("server config", zap.Any("config", serverCfg))

					s, err := server.New(serverCfg, cmdFactory, logger)
					if err != nil {
						return err
					}

					// When using a unix socket, we want to create a file with server's PID.
					if path := pidFileNameFromAddr(cfg.Server.Address); path != "" {
						logger.Debug("creating PID file", zap.String("path", path))
						if err := createFileWithPID(path); err != nil {
							return errors.WithStack(err)
						}
						defer os.Remove(cfg.Server.Address)
					}

					logger.Debug("starting the server")

					return errors.WithStack(s.Serve())
				},
			)
		},
	}

	return &cmd
}

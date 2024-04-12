package server

import (
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/stateful/runme/v3/internal/config"
	"github.com/stateful/runme/v3/internal/config/autoconfig"
	"github.com/stateful/runme/v3/internal/server"
)

func serverStartCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:   "start",
		Short: "Start a server.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return autoconfig.Invoke(
				func(
					cfg *config.Config,
					logger *zap.Logger,
				) error {
					defer logger.Sync()

					serverCfg := &server.Config{
						Address:    cfg.Core.ServerAddress,
						CertFile:   cfg.Core.ServerTLSCertFile,
						KeyFile:    cfg.Core.ServerTLSKeyFile,
						TLSEnabled: cfg.Core.ServerTLSEnabled,
					}

					logger.Debug("server config", zap.Any("config", serverCfg))

					s, err := server.New(serverCfg, logger)
					if err != nil {
						return err
					}

					// When using a unix socket, we want to create a file with server's PID.
					if path := pidFileNameFromAddr(cfg.Core.ServerAddress); path != "" {
						logger.Debug("creating PID file", zap.String("path", path))
						if err := createFileWithPID(path); err != nil {
							return errors.WithStack(err)
						}
						defer os.Remove(cfg.Core.ServerAddress)
					}

					return errors.WithStack(s.Serve())
				},
			)
		},
	}

	return &cmd
}

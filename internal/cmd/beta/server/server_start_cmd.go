package server

import (
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

					serverCfg := server.Config{
						Address:    cfg.ServerAddress,
						CertFile:   cfg.ServerTLSCertFile,
						KeyFile:    cfg.ServerTLSKeyFile,
						TLSEnabled: cfg.ServerTLSEnabled,
					}

					s, err := server.New(&serverCfg, logger)
					if err != nil {
						return err
					}

					return errors.WithStack(s.Serve())
				},
			)
		},
	}

	return &cmd
}

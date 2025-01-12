package server

import (
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

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
			return autoconfig.Invoke(
				func(
					cfg *config.Config,
					server *server.Server,
					logger *zap.Logger,
				) error {
					defer logger.Sync()

					_ = telemetry.ReportUnlessNoTracking(logger)

					// When using a unix socket, we want to create a file with server's PID.
					if path := pidFileNameFromAddr(cfg.Server.Address); path != "" {
						logger.Debug("creating PID file", zap.String("path", path))
						if err := createFileWithPID(path); err != nil {
							return errors.WithStack(err)
						}
						defer os.Remove(cfg.Server.Address)
					}

					return errors.WithStack(server.Serve())
				},
			)
		},
	}

	return &cmd
}

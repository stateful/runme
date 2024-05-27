package server

import (
	"context"
	"strings"

	"github.com/fullstorydev/grpcurl"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/stateful/runme/v3/internal/config"
	"github.com/stateful/runme/v3/internal/config/autoconfig"
)

func serverGRPCurlListCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:   "list",
		Short: "List gRPC services exposed by the server.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return autoconfig.InvokeForCommand(
				func(
					cfg *config.Config,
					logger *zap.Logger,
				) error {
					defer logger.Sync()

					var (
						result []string
						err    error
					)

					switch len(args) {
					case 1:
						result, err = listMethods(cmd.Context(), cfg, args[0])
					case 0:
						result, err = listServices(cmd.Context(), cfg)
					}
					if err != nil {
						return err
					}

					_, err = cmd.OutOrStdout().Write([]byte(strings.Join(result, "\n")))
					if err != nil {
						return errors.WithStack(err)
					}
					_, err = cmd.OutOrStdout().Write([]byte("\n"))
					return errors.WithStack(err)
				},
			)
		},
	}

	return &cmd
}

func listMethods(ctx context.Context, cfg *config.Config, symbol string) ([]string, error) {
	descSource, err := getDescriptorSource(ctx, cfg)
	if err != nil {
		return nil, err
	}
	methods, err := grpcurl.ListMethods(descSource, symbol)
	return methods, errors.WithStack(err)
}

func listServices(ctx context.Context, cfg *config.Config) ([]string, error) {
	descSource, err := getDescriptorSource(ctx, cfg)
	if err != nil {
		return nil, err
	}
	services, err := grpcurl.ListServices(descSource)
	return services, errors.WithStack(err)
}

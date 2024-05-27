package server

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/fullstorydev/grpcurl"
	"github.com/jhump/protoreflect/grpcreflect"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stateful/runme/v3/internal/config"
	"github.com/stateful/runme/v3/internal/config/autoconfig"
)

func serverGRPCurlInvokeCmd() *cobra.Command {
	var data string

	cmd := cobra.Command{
		Use:   "invoke",
		Short: "Invoke gRPC command to the server.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return autoconfig.InvokeForCommand(
				func(
					cfg *config.Config,
					logger *zap.Logger,
				) error {
					defer logger.Sync()

					var in io.Reader
					if data == "@" {
						in = cmd.InOrStdin()
					} else {
						in = strings.NewReader(data)
					}

					return invokeRPCAndPrintResult(cmd.Context(), cfg, in, cmd.OutOrStdout(), args[0])
				},
			)
		},
	}

	cmd.Flags().StringVarP(&data, "data", "d", "", "Data to send to the server.")

	return &cmd
}

func invokeRPCAndPrintResult(ctx context.Context, cfg *config.Config, in io.Reader, out io.Writer, symbol string) error {
	cc, err := dialServer(ctx, cfg)
	if err != nil {
		return err
	}
	defer cc.Close()

	client := grpcreflect.NewClientAuto(ctx, cc)
	descSource := grpcurl.DescriptorSourceFromServer(ctx, client)

	options := grpcurl.FormatOptions{
		EmitJSONDefaultFields: true,
		AllowUnknownFields:    true,
	}
	parser, formatter, err := grpcurl.RequestParserAndFormatter(defaultGRPCurlFormat, descSource, in, options)
	if err != nil {
		return errors.WithStack(err)
	}

	var headers []string

	eventHandler := &grpcurl.DefaultEventHandler{
		Out:            out,
		Formatter:      formatter,
		VerbosityLevel: 0,
	}
	err = grpcurl.InvokeRPC(ctx, descSource, cc, symbol, headers, eventHandler, parser.Next)
	if err != nil {
		errStatus, ok := status.FromError(err)
		if !ok {
			return errors.Wrapf(err, "error invoking method %q; failed to extract status", symbol)
		}
		eventHandler.Status = errStatus
	}

	reqSuffix, respSuffix := "", ""
	reqCount := parser.NumRequests()
	if reqCount != 1 {
		reqSuffix = "s"
	}
	respCount := eventHandler.NumResponses
	if respCount != 1 {
		respSuffix = "s"
	}
	_, err = fmt.Fprintf(out, "Sent %d request%s and received %d response%s\n", reqCount, reqSuffix, respCount, respSuffix)
	if err != nil {
		return errors.WithStack(err)
	}

	if eventHandler.Status.Code() != codes.OK {
		grpcurl.PrintStatus(out, eventHandler.Status, formatter)
	}

	return nil
}

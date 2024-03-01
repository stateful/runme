package server

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/fullstorydev/grpcurl"
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
			return autoconfig.Invoke(
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

					return invokeRPC(cmd.Context(), cfg, in, cmd.OutOrStdout(), args[0])
				},
			)
		},
	}

	cmd.Flags().StringVarP(&data, "data", "d", "", "Data to send to the server.")

	return &cmd
}

func invokeRPC(ctx context.Context, cfg *config.Config, in io.Reader, out io.Writer, symbol string) error {
	cc, err := dialServer(ctx, cfg)
	if err != nil {
		return err
	}

	descSource, err := getDescSource(ctx, cfg)
	if err != nil {
		return err
	}

	options := grpcurl.FormatOptions{
		EmitJSONDefaultFields: true,
		IncludeTextSeparator:  true,
		AllowUnknownFields:    true,
	}

	rf, formatter, err := grpcurl.RequestParserAndFormatter(grpcurl.Format("json"), descSource, in, options)
	if err != nil {
		return errors.WithStack(err)
	}

	h := &grpcurl.DefaultEventHandler{
		Out:            out,
		Formatter:      formatter,
		VerbosityLevel: 0,
	}

	err = grpcurl.InvokeRPC(ctx, descSource, cc, symbol, nil, h, rf.Next)
	if err != nil {
		errStatus, ok := status.FromError(err)
		if !ok {
			return errors.Wrapf(err, "error invoking method %q", symbol)
		}
		h.Status = errStatus
	}

	reqSuffix := ""
	respSuffix := ""
	reqCount := rf.NumRequests()
	if reqCount != 1 {
		reqSuffix = "s"
	}
	if h.NumResponses != 1 {
		respSuffix = "s"
	}
	_, err = fmt.Fprintf(out, "Sent %d request%s and received %d response%s\n", reqCount, reqSuffix, h.NumResponses, respSuffix)
	if err != nil {
		return errors.WithStack(err)
	}

	if h.Status.Code() != codes.OK {
		grpcurl.PrintStatus(out, h.Status, formatter)
	}

	return nil
}

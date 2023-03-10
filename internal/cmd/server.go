package cmd

import (
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/bufbuild/connect-go"
	grpchealth "github.com/bufbuild/connect-grpchealth-go"
	grpcreflect "github.com/bufbuild/connect-grpcreflect-go"
	"github.com/rs/cors"
	"github.com/spf13/cobra"
	"github.com/stateful/runme/internal/document/editor/editorservice"
	parserv1 "github.com/stateful/runme/internal/gen/proto/go/runme/parser/v1"
	runnerv1 "github.com/stateful/runme/internal/gen/proto/go/runme/runner/v1"
	"github.com/stateful/runme/internal/gen/proto/go/runme/runner/v1/runnerv1connect"
	"github.com/stateful/runme/internal/runner"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func serverCmd() *cobra.Command {
	const (
		defaultSocketAddr = "unix:///var/run/runme.sock"
		defaultLocalAddr  = "localhost:7890"
	)

	var (
		addr               string
		useConnectProtocol bool
		devMode            bool
		enableRunner       bool
	)

	cmd := cobra.Command{
		Hidden: true,
		Use:    "server",
		Short:  "Start a server with various services and a gRPC interface.",
		Long: `The server provides two services: kernel and parser.

The parser allows serializing and deserializing markdown content.

The kernel is used to run long running processes like shells and interacting with them.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			var (
				logger *zap.Logger
				err    error
			)
			if devMode {
				logger, err = zap.NewDevelopment()
			} else {
				logger, err = zap.NewProduction()
			}
			if err != nil {
				return err
			}
			defer logger.Sync()

			// When web is true, the server command exposes a gRPC-compatible HTTP API.
			// Read more on https://connect.build/docs/introduction.
			if useConnectProtocol {
				if addr == defaultSocketAddr {
					addr = defaultLocalAddr
				}

				mux := http.NewServeMux()
				compress1KB := connect.WithCompressMinBytes(1024)
				if enableRunner {
					runnerService, err := runner.NewRunnerServiceHandler(logger)
					if err != nil {
						return err
					}
					mux.Handle(runnerv1connect.NewRunnerServiceHandler(runnerService))
				}
				mux.Handle(grpchealth.NewHandler(
					grpchealth.NewStaticChecker(),
					compress1KB,
				))
				mux.Handle(grpcreflect.NewHandlerV1(
					grpcreflect.NewStaticReflector(),
					compress1KB,
				))
				mux.Handle(grpcreflect.NewHandlerV1Alpha(
					grpcreflect.NewStaticReflector(),
					compress1KB,
				))

				srv := &http.Server{
					Addr: addr,
					Handler: h2c.NewHandler(
						newCORS().Handler(mux),
						&http2.Server{},
					),
					ReadHeaderTimeout: time.Second,
					ReadTimeout:       5 * time.Minute,
					WriteTimeout:      5 * time.Minute,
					MaxHeaderBytes:    8 * 1024, // 8KiB
				}

				logger.Info("started listening", zap.String("addr", srv.Addr))

				return srv.ListenAndServe()
			}

			var lis net.Listener

			if strings.HasPrefix(addr, "unix://") {
				addr := strings.TrimPrefix(addr, "unix://")

				// TODO: consolidate removing address into a single place
				_ = os.Remove(addr)

				lis, err = net.Listen("unix", addr)
				if err != nil {
					return err
				}
				defer func() { _ = os.Remove(addr) }()
			} else {
				lis, err = net.Listen("tcp", addr)
				if err != nil {
					return err
				}
			}

			logger.Info("started listening", zap.String("addr", lis.Addr().String()))

			server := grpc.NewServer(
				grpc.MaxRecvMsgSize(runner.MaxMsgSize),
				grpc.MaxSendMsgSize(runner.MaxMsgSize),
			)
			parserv1.RegisterParserServiceServer(server, editorservice.NewParserServiceServer(logger))
			if enableRunner {
				runnerService, err := runner.NewRunnerService(logger)
				if err != nil {
					return err
				}
				runnerv1.RegisterRunnerServiceServer(server, runnerService)
			}
			reflection.Register(server)
			return server.Serve(lis)
		},
	}

	setDefaultFlags(&cmd)

	cmd.Flags().StringVarP(&addr, "address", "a", defaultSocketAddr, "Address to create unix (unix:///path/to/socket) or IP socket (localhost:7890)")
	cmd.Flags().BoolVar(&useConnectProtocol, "connect-protocol", false, "Use Connect Protocol (https://connect.build/)")
	cmd.Flags().BoolVar(&devMode, "dev", false, "Enable development mode")
	cmd.Flags().BoolVar(&enableRunner, "runner", false, "Enable runner service")

	return &cmd
}

func newCORS() *cors.Cors {
	return cors.New(cors.Options{
		AllowedMethods: []string{
			http.MethodHead,
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
		},
		AllowOriginFunc: func(origin string) bool {
			// Allow all origins, which effectively disables CORS.
			return true
		},
		AllowedHeaders: []string{"*"},
		ExposedHeaders: []string{
			// Content-Type is in the default safelist.
			"Accept",
			"Accept-Encoding",
			"Accept-Post",
			"Connect-Accept-Encoding",
			"Connect-Content-Encoding",
			"Content-Encoding",
			"Grpc-Accept-Encoding",
			"Grpc-Encoding",
			"Grpc-Message",
			"Grpc-Status",
			"Grpc-Status-Details-Bin",
		},
		// Let browsers cache CORS information for longer, which reduces the number
		// of preflight requests. Any changes to ExposedHeaders won't take effect
		// until the cached data expires. FF caps this value at 24h, and modern
		// Chrome caps it at 2h.
		MaxAge: int(2 * time.Hour / time.Second),
	})
}

package cmd

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"time"

	"github.com/bufbuild/connect-go"
	grpchealth "github.com/bufbuild/connect-grpchealth-go"
	grpcreflect "github.com/bufbuild/connect-grpcreflect-go"
	"github.com/rs/cors"
	"github.com/spf13/cobra"
	"github.com/stateful/runme/internal/document/editor"
	kernelv1 "github.com/stateful/runme/internal/gen/proto/go/runme/kernel/v1"
	"github.com/stateful/runme/internal/gen/proto/go/runme/kernel/v1/kernelv1connect"
	parserv1 "github.com/stateful/runme/internal/gen/proto/go/runme/parser/v1"
	"github.com/stateful/runme/internal/kernel"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
)

func serverCmd() *cobra.Command {
	const defaultSocketAddr = "/var/run/runme.sock"

	var (
		addr string
		web  bool
	)

	cmd := cobra.Command{
		Use:   "server",
		Short: "Start a server with various services and a gRPC interface.",
		Long: `The server provides two services: kernel and parser.

The parser allows serializing and deserializing markdown content.

The kernel is used to run long running processes like shells and interacting with them.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger, err := zap.NewDevelopment()
			if err != nil {
				return err
			}
			defer logger.Sync()

			// When web is true, the server command exposes a gRPC-compatible HTTP API.
			// Read more on https://connect.build/docs/introduction.
			if web {
				if addr == defaultSocketAddr {
					addr = "localhost:8080"
				}

				mux := http.NewServeMux()
				compress1KB := connect.WithCompressMinBytes(1024)
				mux.Handle(kernelv1connect.NewKernelServiceHandler(kernel.NewKernelServiceHandler(logger)))
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

			lis, err := net.Listen("unix", addr)
			if err != nil {
				return err
			}

			logger.Info("started listening", zap.String("addr", lis.Addr().String()))

			var opts []grpc.ServerOption
			server := grpc.NewServer(opts...)
			parserv1.RegisterParserServiceServer(server, &parserServiceServer{})
			kernelv1.RegisterKernelServiceServer(server, kernel.NewKernelServiceServer(logger))
			return server.Serve(lis)
		},
	}

	setDefaultFlags(&cmd)

	cmd.Flags().StringVarP(&addr, "address", "a", defaultSocketAddr, "A path to create a socket.")
	cmd.Flags().BoolVar(&web, "web", false, "Use Connect Protocol (https://connect.build/).")

	return &cmd
}

type parserServiceServer struct {
	parserv1.UnimplementedParserServiceServer
}

func (s *parserServiceServer) Deserialize(_ context.Context, req *parserv1.DeserializeRequest) (*parserv1.DeserializeResponse, error) {
	notebook, err := editor.Deserialize(req.Source)
	if err != nil {
		return nil, err
	}

	cells := make([]*parserv1.Cell, 0, len(notebook.Cells))
	for _, cell := range notebook.Cells {
		cells = append(cells, &parserv1.Cell{
			Kind:     parserv1.CellKind(cell.Kind),
			Value:    cell.Value,
			LangId:   cell.LangID,
			Metadata: stringifyMapValue(cell.Metadata),
		})
	}

	return &parserv1.DeserializeResponse{
		Notebook: &parserv1.Notebook{
			Cells:    cells,
			Metadata: stringifyMapValue(notebook.Metadata),
		},
	}, nil
}

func (s *parserServiceServer) Serialize(_ context.Context, req *parserv1.SerializeRequest) (*parserv1.SerializeResponse, error) {
	cells := make([]*editor.Cell, 0, len(req.Notebook.Cells))
	for _, cell := range req.Notebook.Cells {
		cells = append(cells, &editor.Cell{
			Kind:     editor.CellKind(cell.Kind),
			Value:    cell.Value,
			LangID:   cell.LangId,
			Metadata: parseMapValue(cell.Metadata),
		})
	}
	data, err := editor.Serialize(&editor.Notebook{
		Cells:    cells,
		Metadata: parseMapValue(req.Notebook.Metadata),
	})
	if err != nil {
		return nil, err
	}
	return &parserv1.SerializeResponse{Result: data}, nil
}

func stringifyMapValue(meta map[string]any) map[string]string {
	result := make(map[string]string, len(meta))
	for k, v := range meta {
		rawValue, _ := json.Marshal(v)
		result[k] = string(rawValue)
	}
	return result
}

func parseMapValue(meta map[string]string) map[string]any {
	result := make(map[string]any, len(meta))
	for k, v := range meta {
		result[k] = v
	}
	return result
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

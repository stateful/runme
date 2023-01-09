package cmd

import (
	"context"
	"encoding/json"
	"net"
	"os"

	"github.com/spf13/cobra"
	"github.com/stateful/runme/internal/document/editor"
	kernelv1 "github.com/stateful/runme/internal/gen/proto/go/runme/kernel/v1"
	parserv1 "github.com/stateful/runme/internal/gen/proto/go/runme/parser/v1"
	"github.com/stateful/runme/internal/kernel"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func serverCmd() *cobra.Command {
	var addr string

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

			// TODO: figure out better handling of the socket creation and removal.
			_ = os.Remove(addr)

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

	cmd.Flags().StringVarP(&addr, "address", "a", "/var/run/runme.sock", "Address to bind server.")

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

package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"

	"github.com/spf13/cobra"
	"github.com/stateful/runme/internal/document/editor"
	kernelv1 "github.com/stateful/runme/internal/gen/proto/go/kernel/v1"
	runmev1 "github.com/stateful/runme/internal/gen/proto/go/runme/v1"
	"github.com/stateful/runme/internal/kernel"
	"google.golang.org/grpc"
)

func daemonCmd() *cobra.Command {
	var addr string

	cmd := cobra.Command{
		Use:   "daemon",
		Short: "Start a daemon with a shell session.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = os.Remove(addr)

			lis, err := net.Listen("unix", addr)
			if err != nil {
				return err
			}
			var opts []grpc.ServerOption
			server := grpc.NewServer(opts...)
			runmev1.RegisterRunmeServiceServer(server, &runmeServiceServer{})
<<<<<<< HEAD
			printf("starting listen on %s", addr)
=======
			kernelv1.RegisterKernelServiceServer(server, &kernel.KernelServiceServer{})
>>>>>>> e482ff7 (kernel: implement execution with writer)
			return server.Serve(lis)
		},
	}

	setDefaultFlags(&cmd)

	cmd.Flags().StringVarP(&addr, "address", "a", "/var/run/runme.sock", "Address to bind server.")

	return &cmd
}

type runmeServiceServer struct {
	runmev1.UnimplementedRunmeServiceServer
}

func (s *runmeServiceServer) Deserialize(_ context.Context, req *runmev1.DeserializeRequest) (*runmev1.DeserializeResponse, error) {
	notebook, err := editor.Deserialize(req.Source)
	if err != nil {
		return nil, err
	}

	cells := make([]*runmev1.Cell, 0, len(notebook.Cells))
	for _, cell := range notebook.Cells {
		cells = append(cells, &runmev1.Cell{
			Kind:     runmev1.CellKind(cell.Kind),
			Value:    cell.Value,
			LangId:   cell.LangID,
			Metadata: stringifyMapValue(cell.Metadata),
		})
	}

	return &runmev1.DeserializeResponse{
		Notebook: &runmev1.Notebook{
			Cells:    cells,
			Metadata: stringifyMapValue(notebook.Metadata),
		},
	}, nil
}

func (s *runmeServiceServer) Serialize(_ context.Context, req *runmev1.SerializeRequest) (*runmev1.SerializeResponse, error) {
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
	return &runmev1.SerializeResponse{Result: data}, nil
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

func printf(msg string, args ...any) {
	var buf bytes.Buffer
	_, _ = buf.WriteString("\x1b[0;32m")
	_, _ = fmt.Fprintf(&buf, msg, args...)
	_, _ = buf.WriteString("\x1b[0m")
	_, _ = buf.WriteString("\r\n")
	_, _ = os.Stderr.Write(buf.Bytes())
}

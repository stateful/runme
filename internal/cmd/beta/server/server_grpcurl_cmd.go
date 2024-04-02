package server

import (
	"github.com/spf13/cobra"
)

func serverGRPCurlCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:   "grpcurl",
		Short: "Interact with the server using grpcurl.",
	}

	cmd.AddCommand(serverGRPCurlDescribeCmd())
	cmd.AddCommand(serverGRPCurlInvokeCmd())
	cmd.AddCommand(serverGRPCurlListCmd())

	return &cmd
}

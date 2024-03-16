package cmd

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/cli/go-gh/pkg/tableprinter"
	"github.com/pkg/errors"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	runnerv1 "github.com/stateful/runme/v3/internal/gen/proto/go/runme/runner/v1"
	runmetls "github.com/stateful/runme/v3/internal/tls"
)

var osEnviron = os.Environ

func environmentCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:     "env",
		Aliases: []string{"environment"},
		Hidden:  true,
		Short:   "Environment management",
		Long:    "Various commands to manage environments in runme",
	}

	cmd.AddCommand(environmentDumpCmd())
	cmd.AddCommand(storeCmd())

	setDefaultFlags(&cmd)

	return &cmd
}

func storeCmd() *cobra.Command {
	cmd := cobra.Command{
		Hidden: true,
		Use:    "store",
		Short:  "Owl store",
		Long:   "Owl Store",
	}

	cmd.AddCommand(storeSnapshotCmd())

	return &cmd
}

func storeSnapshotCmd() *cobra.Command {
	var (
		addr      string
		tlsDir    string
		sessionID string
	)

	cmd := cobra.Command{
		Hidden: true,
		Use:    "snapshot",
		Short:  "Dump environment variables to stdout",
		Long:   "Dumps all environment variables to stdout as a table",
		RunE: func(cmd *cobra.Command, args []string) error {
			tlsConfig, err := runmetls.LoadTLSConfig(tlsDir, true)
			if err != nil {
				return err
			}

			credentials := credentials.NewTLS(tlsConfig)

			conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(credentials))
			if err != nil {
				return errors.Wrap(err, "failed to connect")
			}
			defer conn.Close()

			client := runnerv1.NewRunnerServiceClient(conn)

			meClient, err := client.MonitorEnvStore(context.Background(), &runnerv1.MonitorEnvStoreRequest{
				Session: &runnerv1.Session{Id: sessionID},
			})
			if err != nil {
				return err
			}

			var msg runnerv1.MonitorEnvStoreResponse
			err = meClient.RecvMsg(&msg)
			if err != nil {
				return err
			}

			if msgData, ok := msg.Data.(*runnerv1.MonitorEnvStoreResponse_Snapshot); ok {
				return errors.Wrap(printStore(*msgData), "failed to render")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&addr, "address", os.Getenv("RUNME_SERVER_ADDR"), "The Server address to connect to, i.e. 127.0.0.1:7865")
	cmd.Flags().StringVar(&tlsDir, "tlsDir", os.Getenv("RUNME_TLS_DIR"), "Path to tls files")
	cmd.Flags().StringVar(&sessionID, "session", os.Getenv("RUNME_SESSION"), "Session Id")

	return &cmd
}

func environmentDumpCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:   "dump",
		Short: "Dump environment variables to stdout",
		Long:  "Dumps all environment variables to stdout as a list of K=V separated by null terminators",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !fInsecure {
				return errors.New("must be run in insecure mode; enable by running with --insecure flag")
			}

			dumped := getDumpedEnvironment()

			_, _ = cmd.OutOrStdout().Write([]byte(dumped))

			return nil
		},
	}

	setDefaultFlags(&cmd)

	return &cmd
}

func printStore(msgData runnerv1.MonitorEnvStoreResponse_Snapshot) error {
	io := iostreams.System()

	// TODO: Clear terminal screen

	table := tableprinter.New(io.Out, io.IsStdoutTTY(), io.TerminalWidth())
	table.AddField(strings.ToUpper("Name"))
	table.AddField(strings.ToUpper("Value"))
	table.AddField(strings.ToUpper("Spec"))
	table.AddField(strings.ToUpper("Origin"))
	table.AddField(strings.ToUpper("Updated"))
	table.EndRow()

	for _, env := range msgData.Snapshot.Envs {
		value := env.ResolvedValue

		if env.Status.Number() == runnerv1.MonitorEnvStoreResponseSnapshot_STATUS_MASKED.Number() {
			value = "[masked]"
		} else if env.Status.Number() == runnerv1.MonitorEnvStoreResponseSnapshot_STATUS_HIDDEN.Number() {
			value = "****"
		}

		table.AddField(env.Name)
		table.AddField(value)
		table.AddField(env.Spec)
		table.AddField(env.Origin)

		t, err := time.Parse(time.RFC3339, env.UpdateTime)
		if err == nil {
			table.AddField(t.Format(time.RFC1123Z))
		} else {
			table.AddField("-")
		}

		table.EndRow()
	}

	return table.Render()
}

func getDumpedEnvironment() string {
	return strings.Join(osEnviron(), "\x00")
}

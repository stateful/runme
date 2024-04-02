package cmd

import (
	"context"
	"fmt"
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
	"github.com/stateful/runme/v3/internal/runner/client"
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
	cmd.AddCommand(storeCheckCmd())

	return &cmd
}

func storeSnapshotCmd() *cobra.Command {
	var (
		addr      string
		tlsDir    string
		sessionID string
		limit     int
		all       bool
	)

	cmd := cobra.Command{
		Hidden: true,
		Use:    "snapshot",
		Short:  "Takes a snapshot of the smart env store",
		Long:   "Connects with a running server to inspect the environment variables of a session and returns a snapshot of the smart env store.",
		RunE: func(cmd *cobra.Command, args []string) error {
			tlsConfig, err := runmetls.LoadClientConfigFromDir(tlsDir)
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
				return errors.Wrap(printStore(msgData, limit, all), "failed to render")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&addr, "address", os.Getenv("RUNME_SERVER_ADDR"), "The Server address to connect to, i.e. 127.0.0.1:7865")
	cmd.Flags().StringVar(&tlsDir, "tlsDir", os.Getenv("RUNME_TLS_DIR"), "Path to tls files")
	cmd.Flags().StringVar(&sessionID, "session", os.Getenv("RUNME_SESSION"), "Session Id")
	cmd.Flags().IntVar(&limit, "limit", 15, "Limit the number of lines")
	cmd.Flags().BoolVarP(&all, "all", "A", false, "Show all lines")

	return &cmd
}

func storeCheckCmd() *cobra.Command {
	var (
		serverAddr    string
		getRunnerOpts func() ([]client.RunnerOption, error)
	)

	cmd := cobra.Command{
		Hidden: true,
		Use:    "check",
		Short:  "Validates smart store",
		Long:   "Connects with a running server to validates smart store, exiting with success or displaying API errors.",
		RunE: func(cmd *cobra.Command, args []string) error {
			project, err := getProject()
			if err != nil {
				return err
			}

			runnerOpts, err := getRunnerOpts()
			if err != nil {
				return err
			}

			runnerOpts = append(
				runnerOpts,
				client.WithinShellMaybe(),
				client.WithStdin(cmd.InOrStdin()),
				client.WithCleanupSession(true),
				client.WithStdout(cmd.OutOrStdout()),
				client.WithStderr(cmd.ErrOrStderr()),
				client.WithProject(project),
				client.WithEnvStoreType(runnerv1.SessionEnvStoreType_SESSION_ENV_STORE_TYPE_OWL),
			)

			_, err = client.NewRemoteRunner(
				cmd.Context(),
				serverAddr,
				runnerOpts...,
			)
			if err != nil {
				// todo(sebastian): hack
				errStr := err.Error()
				parts := strings.Split(errStr, "Unknown desc = ")
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Error: %s\n", parts[len(parts)-1])
				return nil
			}

			// _, err = fmt.Printf("session created successfully in %s with id %s\n", project.Root(), runner.GetSessionID())
			_, err = fmt.Println("Success")
			return err
		},
	}

	getRunnerOpts = setRunnerFlags(&cmd, &serverAddr)

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

func printStore(msgData *runnerv1.MonitorEnvStoreResponse_Snapshot, lines int, all bool) error {
	io := iostreams.System()

	// TODO: Clear terminal screen

	table := tableprinter.New(io.Out, io.IsStdoutTTY(), io.TerminalWidth())
	table.AddField(strings.ToUpper("Name"))
	table.AddField(strings.ToUpper("Value"))
	table.AddField(strings.ToUpper("Spec"))
	table.AddField(strings.ToUpper("Origin"))
	table.AddField(strings.ToUpper("Updated"))
	table.EndRow()

	for i, env := range msgData.Snapshot.Envs {
		if i >= lines && !all {
			break
		}
		value := env.ResolvedValue

		switch env.Status {
		case runnerv1.MonitorEnvStoreResponseSnapshot_STATUS_UNSPECIFIED:
			value = "[unset]"
		case runnerv1.MonitorEnvStoreResponseSnapshot_STATUS_MASKED:
			value = "[masked]"
		case runnerv1.MonitorEnvStoreResponseSnapshot_STATUS_HIDDEN:
			value = "******"
		}

		table.AddField(env.Name)
		table.AddField(value)
		table.AddField(env.Spec)
		table.AddField(env.Origin)

		t, err := time.Parse(time.RFC3339, env.UpdateTime)
		if err == nil {
			table.AddField(t.Format(time.DateTime))
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

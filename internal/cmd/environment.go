package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/cli/go-gh/pkg/tableprinter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/stateful/runme/v3/internal/command"
	"github.com/stateful/runme/v3/internal/owl"
	"github.com/stateful/runme/v3/internal/runner/client"
	runmetls "github.com/stateful/runme/v3/internal/tls"
	runnerv1 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/runner/v1"
)

var newOSEnvironReader = func() (io.Reader, error) {
	return command.NewEnvProducerFromEnv()
}

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
		serverAddr      string
		sessionID       string
		sessionStrategy string
		tlsDir          string
		limit           int
		all             bool
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
			conn, err := grpc.Dial(serverAddr, grpc.WithTransportCredentials(credentials))
			if err != nil {
				return errors.Wrap(err, "failed to connect")
			}
			defer conn.Close()

			client := runnerv1.NewRunnerServiceClient(conn)

			// todo(sebastian): this should move into API as part of v2
			if strings.ToLower(sessionStrategy) == "recent" {
				req := &runnerv1.ListSessionsRequest{}
				resp, err := client.ListSessions(context.Background(), req)
				if err != nil {
					return err
				}
				l := len(resp.Sessions)
				if l == 0 {
					return errors.New("no sessions found")
				}
				// potentially unreliable
				sessionID = resp.Sessions[l-1].Id
			}

			req := &runnerv1.MonitorEnvStoreRequest{
				Session: &runnerv1.Session{Id: sessionID},
			}
			meClient, err := client.MonitorEnvStore(context.Background(), req)
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

	cmd.Flags().StringVar(&serverAddr, "server-address", os.Getenv("RUNME_SERVER_ADDR"), "The Server ServerAddress to connect to, i.e. 127.0.0.1:7865")
	cmd.Flags().StringVar(&tlsDir, "tls-dir", os.Getenv("RUNME_TLS_DIR"), "Path to tls files")
	cmd.Flags().StringVar(&sessionID, "session", os.Getenv("RUNME_SESSION"), "Session Id")
	cmd.Flags().StringVar(&sessionStrategy, "session-strategy", func() string {
		if val, ok := os.LookupEnv("RUNME_SESSION_STRATEGY"); ok {
			return val
		}

		return "manual"
	}(), "Strategy for session selection. Options are manual, recent. Defaults to manual")
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
				return errors.New("must be run in insecure mode to prevent misuse; enable by adding --insecure flag")
			}

			producer, err := newOSEnvironReader()
			if err != nil {
				return err
			}

			_, _ = io.Copy(cmd.OutOrStdout(), producer)

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

		specless := msgData.Snapshot.Envs[0].Spec != owl.SpecNameOpaque
		if !all && specless && env.Spec == owl.SpecNameOpaque {
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
		stripped := strings.ReplaceAll(strings.ReplaceAll(value, "\n", " "), "\r", "")
		table.AddField(stripped)
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

package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/cli/go-gh/pkg/tableprinter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/stateful/runme/v3/internal/command"
	"github.com/stateful/runme/v3/internal/owl"
	"github.com/stateful/runme/v3/internal/runner/client"
	"github.com/stateful/runme/v3/internal/term"
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

type envStoreFlags struct {
	serverAddr      string
	sessionID       string
	sessionStrategy string
	tlsDir          string
}

func storeCmd() *cobra.Command {
	var storeFlags envStoreFlags

	cmd := cobra.Command{
		Hidden: true,
		Use:    "store",
		Short:  "Owl store",
		Long:   "Owl Store",
	}

	cmd.Flags().StringVar(&storeFlags.serverAddr, "server-address", os.Getenv("RUNME_SERVER_ADDR"), "The Server ServerAddress to connect to, i.e. 127.0.0.1:7865")
	cmd.Flags().StringVar(&storeFlags.tlsDir, "tls-dir", os.Getenv("RUNME_TLS_DIR"), "Path to tls files")
	cmd.Flags().StringVar(&storeFlags.sessionID, "session", os.Getenv("RUNME_SESSION"), "Session Id")
	cmd.Flags().StringVar(&storeFlags.sessionStrategy, "session-strategy", func() string {
		if val, ok := os.LookupEnv("RUNME_SESSION_STRATEGY"); ok {
			return val
		}
		return "manual"
	}(), "Strategy for session selection. Options are manual, recent. Defaults to manual")

	cmd.AddCommand(storeSnapshotCmd(storeFlags))
	cmd.AddCommand(storeSourceCmd(storeFlags))
	cmd.AddCommand(storeCheckCmd())

	return &cmd
}

func storeSourceCmd(storeFlags envStoreFlags) *cobra.Command {
	var (
		insecure  bool
		export    bool
		sessionID = ""
	)

	cmd := cobra.Command{
		Use:   "source",
		Short: "Source environment variables from session",
		Long:  "Source environment variables from session",
		RunE: func(cmd *cobra.Command, args []string) error {
			// discard any stderr in silent mode
			if !insecure {
				return errors.New("must be run in insecure mode to prevent misuse; enable by adding --insecure flag")
			}

			tlsConfig, err := runmetls.LoadClientConfigFromDir(storeFlags.tlsDir)
			if err != nil {
				return err
			}

			credentials := credentials.NewTLS(tlsConfig)
			conn, err := grpc.NewClient(
				storeFlags.serverAddr,
				grpc.WithTransportCredentials(credentials),
			)
			if err != nil {
				return errors.Wrap(err, "failed to connect")
			}
			defer conn.Close()

			client := runnerv1.NewRunnerServiceClient(conn)

			// todo(sebastian): would it be better to require a specific session?
			if strings.ToLower(storeFlags.sessionStrategy) == "recent" {
				req := &runnerv1.ListSessionsRequest{}
				resp, err := client.ListSessions(cmd.Context(), req)
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

			req := &runnerv1.GetSessionRequest{Id: sessionID}
			resp, err := client.GetSession(cmd.Context(), req)
			if err != nil {
				return err
			}

			for _, kv := range resp.Session.Envs {
				parts := strings.Split(kv, "=")
				if len(parts) < 2 {
					return errors.Errorf("invalid key-value pair: %s", kv)
				}

				envVar := fmt.Sprintf("%s=%q", parts[0], strings.Join(parts[1:], "="))
				if export {
					envVar = fmt.Sprintf("export %s", envVar)
				}

				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\n", envVar); err != nil {
					return err
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&export, "export", "", false, "export variables")
	cmd.Flags().BoolVar(&insecure, "insecure", false, "Explicitly allow delicate operations to prevent misuse")

	return &cmd
}

func storeSnapshotCmd(storeFlags envStoreFlags) *cobra.Command {
	var (
		limit  int
		reveal bool
		all    bool
	)

	cmd := cobra.Command{
		Hidden: true,
		Use:    "snapshot",
		Short:  "Takes a snapshot of the smart env store",
		Long:   "Connects with a running server to inspect the environment variables of a session and returns a snapshot of the smart env store.",
		RunE: func(cmd *cobra.Command, args []string) error {
			tlsConfig, err := runmetls.LoadClientConfigFromDir(storeFlags.tlsDir)
			if err != nil {
				return err
			}

			if reveal && !fInsecure {
				return errors.New("must be run in insecure mode to prevent misuse; enable by adding --insecure flag")
			}

			credentials := credentials.NewTLS(tlsConfig)
			conn, err := grpc.NewClient(
				storeFlags.serverAddr,
				grpc.WithTransportCredentials(credentials),
			)
			if err != nil {
				return errors.Wrap(err, "failed to connect")
			}
			defer conn.Close()

			client := runnerv1.NewRunnerServiceClient(conn)

			// todo(sebastian): this should move into API as part of v2
			if strings.ToLower(storeFlags.sessionStrategy) == "recent" {
				req := &runnerv1.ListSessionsRequest{}
				resp, err := client.ListSessions(cmd.Context(), req)
				if err != nil {
					return err
				}
				l := len(resp.Sessions)
				if l == 0 {
					return errors.New("no sessions found")
				}
				// potentially unreliable
				storeFlags.sessionID = resp.Sessions[l-1].Id
			}

			req := &runnerv1.MonitorEnvStoreRequest{
				Session: &runnerv1.Session{Id: storeFlags.sessionID},
			}
			meClient, err := client.MonitorEnvStore(cmd.Context(), req)
			if err != nil {
				return err
			}

			var msg runnerv1.MonitorEnvStoreResponse
			err = meClient.RecvMsg(&msg)
			if err != nil {
				return err
			}

			if msgData, ok := msg.Data.(*runnerv1.MonitorEnvStoreResponse_Snapshot); ok {
				insecureReveal := reveal && fInsecure
				return errors.Wrap(printStore(cmd, msgData, limit, insecureReveal, all), "failed to render")
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 50, "Limit the number of lines")
	cmd.Flags().BoolVarP(&all, "all", "A", false, "Show all lines")
	cmd.Flags().BoolVarP(&reveal, "reveal", "r", false, "Reveal hidden values")

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

func printStore(cmd *cobra.Command, msgData *runnerv1.MonitorEnvStoreResponse_Snapshot, lines int, reveal, all bool) error {
	term := term.FromIO(cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())

	width, _, err := term.Size()
	if err != nil {
		width = 80
	}

	table := tableprinter.New(term.Out(), term.IsTTY(), width)
	table.AddField(strings.ToUpper("Name"))
	table.AddField(strings.ToUpper("Value"))
	table.AddField(strings.ToUpper("Description"))
	table.AddField(strings.ToUpper("Spec"))
	table.AddField(strings.ToUpper("Source"))
	table.AddField(strings.ToUpper("Updated"))
	table.EndRow()

	specless := true
	for i := range msgData.Snapshot.Envs {
		backwards := msgData.Snapshot.Envs[len(msgData.Snapshot.Envs)-i-1]
		if backwards.Spec != owl.AtomicNameOpaque {
			specless = false
			lines = len(msgData.Snapshot.Envs) - i
			break
		}
	}

	for i, env := range msgData.Snapshot.Envs {
		if i >= lines && !all {
			break
		}

		if !all && specless && env.Spec == owl.AtomicNameOpaque {
			break
		}

		value := env.GetResolvedValue()

		switch env.GetStatus() {
		case runnerv1.MonitorEnvStoreResponseSnapshot_STATUS_UNSPECIFIED:
			value = "[unset]"
		case runnerv1.MonitorEnvStoreResponseSnapshot_STATUS_MASKED:
			value = "[masked]"
		case runnerv1.MonitorEnvStoreResponseSnapshot_STATUS_HIDDEN:
			value = "[hidden]"
			if reveal {
				value = env.GetOriginalValue()
			}
		}

		strippedVal := strings.ReplaceAll(strings.ReplaceAll(value, "\n", " "), "\r", "")

		table.AddField(env.GetName())
		table.AddField(strippedVal)
		table.AddField(env.GetDescription())
		table.AddField(env.GetSpec())
		table.AddField(env.GetOrigin())

		t, err := time.Parse(time.RFC3339, env.GetUpdateTime())
		if err == nil {
			table.AddField(t.Format(time.DateTime))
		} else {
			table.AddField("-")
		}

		table.EndRow()
	}

	return table.Render()
}

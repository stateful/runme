package beta

import (
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	runmetls "github.com/stateful/runme/v3/internal/tls"
	runnerv2 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/runner/v2"
)

func envCmd(cflags *commonFlags) *cobra.Command {
	cmd := cobra.Command{
		Use:     "env",
		Aliases: []string{"environment"},
		Hidden:  true,
		Short:   "Environment management",
		Long:    "Various commands to manage environments in runme",
	}

	cmd.AddCommand(envSourceCmd(cflags))

	return &cmd
}

func envSourceCmd(cflags *commonFlags) *cobra.Command {
	var (
		serverAddr      string
		sessionID       string
		sessionStrategy string
		tlsDir          string
		fExport         bool
	)

	cmd := cobra.Command{
		Use:   "source",
		Short: "Source environment variables from session",
		Long:  "Source environment variables from session",
		RunE: func(cmd *cobra.Command, args []string) error {
			// discard any stderr in silent mode
			if !cflags.insecure {
				return errors.New("must be run in insecure mode to prevent misuse; enable by adding --insecure flag")
			}

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

			client := runnerv2.NewRunnerServiceClient(conn)

			// todo(sebastian): would it be better to require a specific session?
			if strings.ToLower(sessionStrategy) == "recent" {
				req := &runnerv2.ListSessionsRequest{}
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

			req := &runnerv2.GetSessionRequest{Id: sessionID}
			resp, err := client.GetSession(cmd.Context(), req)
			if err != nil {
				return err
			}

			for _, kv := range resp.Session.Env {
				parts := strings.Split(kv, "=")
				if len(parts) < 2 {
					return errors.Errorf("invalid key-value pair: %s", kv)
				}

				envVar := fmt.Sprintf("%s=%q", parts[0], strings.Join(parts[1:], "="))
				if fExport {
					envVar = fmt.Sprintf("export %s", envVar)
				}

				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\n", envVar); err != nil {
					return err
				}
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

	cmd.Flags().BoolVarP(&fExport, "export", "", false, "export variables")

	return &cmd
}

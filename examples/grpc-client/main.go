package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path"

	"github.com/pkg/errors"
	runnerv1 "github.com/stateful/runme/internal/gen/proto/go/runme/runner/v1"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
)

var (
	addr          = flag.String("addr", "127.0.0.1:7890", "the address to connect to")
	file          = flag.String("file", "", "file with content to upper case")
	resultFile    = flag.String("write-result", "-", "path to a result file (default: stdout)")
	createSession = flag.Bool("create-session", false, "create a new session")
	deleteSession = flag.String("delete-session", "", "delete the given session")
	tlsDir        = flag.String("tls", "", "path to tls files")
)

func main() {
	flag.Parse()

	if err := run(); err != nil {
		log.Fatalf("error: %v", err)
	}
}

func run() error {
	credentials, err := loadCredentials()
	if err != nil {
		return err
	}

	conn, err := grpc.Dial(*addr, grpc.WithTransportCredentials(credentials))
	if err != nil {
		return errors.Wrap(err, "failed to connect")
	}
	defer conn.Close()

	healthClient := healthgrpc.NewHealthClient(conn)

	resp, err := healthClient.Check(context.Background(), &healthgrpc.HealthCheckRequest{})
	if err != nil {
		return errors.Wrap(err, "failed to check health")
	}

	if resp.Status != healthgrpc.HealthCheckResponse_SERVING {
		return errors.Errorf("service status: %v", resp.Status)
	}

	client := runnerv1.NewRunnerServiceClient(conn)

	if *createSession {
		resp, err := client.CreateSession(context.Background(), &runnerv1.CreateSessionRequest{
			Envs: os.Environ(),
		})
		if err != nil {
			return err
		}

		_, _ = fmt.Println(resp.Session.Id)

		return nil
	}

	if *deleteSession != "" {
		id := *deleteSession

		_, err := client.DeleteSession(context.Background(), &runnerv1.DeleteSessionRequest{
			Id: id,
		})
		if err != nil {
			return err
		}

		_, _ = fmt.Printf("Successfully deleted session %q", id)

		return nil
	}

	g, ctx := errgroup.WithContext(context.Background())

	stream, err := client.Execute(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to call Execute()")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, "failed to get cwd")
	}

	err = stream.Send(&runnerv1.ExecuteRequest{
		ProgramName: "bash",
		Directory:   cwd,
		Tty:         true,
		Commands:    []string{"tr a-z A-Z"},
	})
	if err != nil {
		return errors.Wrap(err, "failed to send initial request")
	}

	g.Go(func() error {
		source, err := os.Open(*file)
		if err != nil {
			return errors.Wrap(err, "failed to open source file")
		}
		defer func() { _ = source.Close() }()

		buf := make([]byte, 32*1024)

		for readNext := true; readNext; {
			n, err := source.Read(buf)
			if err != nil {
				if !errors.Is(err, io.EOF) {
					return errors.Wrap(err, "failed to read from source")
				}

				buf[0] = 4 // EOT
				n = 1
				readNext = false
			}
			err = stream.Send(&runnerv1.ExecuteRequest{
				InputData: buf[:n],
			})
			if err != nil {
				return errors.Wrap(err, "failed to send msg")
			}
		}

		return nil
	})

	g.Go(func() error {
		var result io.Writer

		if *resultFile == "-" {
			result = os.Stdout
		} else {
			f, err := os.OpenFile(*resultFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
			if err != nil {
				return errors.Wrap(err, "failed to open result file")
			}
			defer func() { _ = f.Close() }()
			result = f
		}

		for {
			msg, err := stream.Recv()
			if err != nil {
				return errors.Wrap(err, "failed to recv msg")
			}

			_, err = result.Write(msg.StdoutData)
			if err != nil {
				return errors.Wrap(err, "failed to write data")
			}

			if len(msg.StderrData) > 0 {
				log.Printf("stderr: %s", msg.StderrData)
			}

			if msg.ExitCode != nil {
				var err error
				if code := msg.ExitCode.Value; code > 0 {
					err = errors.Errorf("command failed with code %d", code)
				}
				return err
			}
		}
	})

	return g.Wait()
}

func loadCredentials() (credentials.TransportCredentials, error) {
	if *tlsDir == "" {
		return insecure.NewCredentials(), nil
	}

	var (
		certPath = path.Join(*tlsDir, "cert.pem")
		pkPath   = path.Join(*tlsDir, "key.pem")
	)

	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, err
	}

	pkPEM, err := os.ReadFile(pkPath)
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(certPEM) {
		return nil, fmt.Errorf("failed to add root certificate to pool")
	}

	cert, err := tls.X509KeyPair(certPEM, pkPEM)
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      certPool,
		MinVersion:   tls.VersionTLS12,
	}

	return credentials.NewTLS(tlsConfig), nil
}

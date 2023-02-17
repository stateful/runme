package main

import (
	"context"
	"flag"
	"io"
	"log"
	"os"

	"github.com/pkg/errors"
	runnerv1 "github.com/stateful/runme/internal/gen/proto/go/runme/runner/v1"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	addr       = flag.String("addr", "127.0.0.1:7890", "the address to connect to")
	file       = flag.String("file", "", "file with content to upper case")
	resultFile = flag.String("write-result", "-", "path to a result file (default: stdout)")
)

func main() {
	flag.Parse()

	if err := run(); err != nil {
		log.Fatalf("error: %v", err)
	}
}

func run() error {
	conn, err := grpc.Dial(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return errors.Wrap(err, "failed to connect")
	}
	defer conn.Close()

	client := runnerv1.NewRunnerServiceClient(conn)

	stream, err := client.Execute(context.Background())
	if err != nil {
		return errors.Wrap(err, "failed to call Execute()")
	}

	var g errgroup.Group

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

	return g.Wait()
}

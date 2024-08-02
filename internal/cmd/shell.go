package cmd

import (
	"bufio"
	"bytes"
	"net"
	"os"
	"os/exec"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/stateful/runme/v3/internal/runner"
)

func shellCmd() *cobra.Command {
	var commandName string

	cmd := cobra.Command{
		Hidden: true,
		Use:    "shell",
		Short:  "Activate runme shell.",
		Long:   "Activate runme shell. This is an experimental feature.",
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			session, err := runner.NewShellSession(commandName)
			if err != nil {
				return errors.Wrap(err, "failed to create shell session")
			}
			defer func() { _ = session.Close() }()

			printfInfo("runme: welcome to runme shell")

			sockPath := "/tmp/runme-" + session.ID() + ".sock"
			var lc net.ListenConfig
			// TODO: context should be closed only after the goroutine below is closed.
			l, err := lc.Listen(cmd.Context(), "unix", sockPath)
			if err != nil {
				return errors.Wrap(err, "failed to listen to sock")
			}
			defer func() { _ = l.Close(); _ = os.Remove(sockPath) }()

			printfInfo("runme: starting backloop communication on %s", sockPath)

			errc := make(chan error, 1)

			go func() {
				for {
					conn, err := l.Accept()
					if err != nil {
						continue
					}

					scanner := bufio.NewScanner(conn)
					scanner.Split(bufio.ScanLines)

					for scanner.Scan() {
						line := scanner.Bytes()
						line = bytes.TrimSpace(line)
						if len(line) == 0 {
							continue
						}
						if err := session.Send(line); err != nil {
							errc <- err
							return
						}
					}

					errc <- scanner.Err()

					_ = conn.Close()
				}
			}()

			defer printfInfo("\nrunme: exiting")

			select {
			case <-session.Done():
				return session.Err()
			case err := <-errc:
				return err
			}
		},
	}

	defaultShell := os.Getenv("SHELL")
	if defaultShell == "" {
		defaultShell, _ = exec.LookPath("bash")
	}
	if defaultShell == "" {
		defaultShell = "/bin/sh"
	}

	cmd.Flags().StringVar(&commandName, "command", defaultShell, "Command to execute and watch")

	return &cmd
}

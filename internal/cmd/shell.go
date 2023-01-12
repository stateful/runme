package cmd

import (
	"bufio"
	"bytes"
	"net"
	"os"
	"os/exec"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/stateful/runme/internal/kernel"
)

func shellCmd() *cobra.Command {
	var (
		commandName string
		promptStr   string
	)

	cmd := cobra.Command{
		Use:   "shell",
		Short: "Activate runme shell.",
		Long:  "Activate runme shell. This is an experimental feature.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			if promptStr == "" {
				prompt, err := kernel.DetectPrompt(commandName)
				if err != nil {
					return errors.Wrap(err, "failed to detect prompt")
				}
				promptStr = string(prompt)
			}

			session, err := kernel.NewShellSession(commandName, promptStr)
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

			errC := make(chan error, 1)

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
							errC <- err
							return
						}
					}

					errC <- scanner.Err()

					_ = conn.Close()
				}
			}()

			defer printfInfo("\nrunme: exiting")

			select {
			case <-session.Done():
				return session.Err()
			case err := <-errC:
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

	cmd.Flags().StringVar(&commandName, "command", defaultShell, "Command to execute and watch.")
	cmd.Flags().StringVar(&promptStr, "prompt", "", "Prompt to use instead of auto detecting it.")

	return &cmd
}

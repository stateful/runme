package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/pkg/errors"
	"github.com/rwtodd/Go.Sed/sed"
	"github.com/spf13/cobra"
	runnerv1 "github.com/stateful/runme/internal/gen/proto/go/runme/runner/v1"
	"github.com/stateful/runme/internal/runner/client"
)

type runCmdOpts struct {
	DryRun          bool
	ReplaceScripts  []string
	ServerAddr      string
	SessionID       string
	SessionStrategy string
}

func runCmd() *cobra.Command {
	opts := runCmdOpts{}

	cmd := cobra.Command{
		Use:               "run",
		Aliases:           []string{"exec"},
		Short:             "Run a selected command",
		Long:              "Run a selected command identified based on its unique parsed name.",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: validCmdNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			blocks, err := getCodeBlocks()
			if err != nil {
				return err
			}

			block, err := lookupCodeBlock(blocks, args[0])
			if err != nil {
				return err
			}

			if err := replace(opts.ReplaceScripts, block.Lines()); err != nil {
				return err
			}

			ctx, cancel := ctxWithSigCancel(cmd.Context())
			defer cancel()

			var stdin io.Reader

			if block.Interactive() {
				// Use pipe here so that it can be closed and the command can exit.
				// Without this approach, the command would hang on reading from stdin.
				r, w := io.Pipe()
				stdin = r
				go func() { _, _ = io.Copy(w, cmd.InOrStdin()) }()
			} else {
				stdin = bytes.NewReader(nil)
			}

			dir, _ := filepath.Abs(fChdir)

			runOpts := []client.RunnerOption{
				client.WithinShellMaybe(),
				client.WithDir(dir),
				client.WithStdin(stdin),
				client.WithStdout(cmd.OutOrStdout()),
				client.WithStderr(cmd.ErrOrStderr()),
				client.WithSessionID(opts.SessionID),
				client.WithCleanupSession(opts.SessionID == ""),
			}

			switch strings.ToLower(opts.SessionStrategy) {
			case "manual":
				runOpts = append(runOpts, client.WithSessionStrategy(runnerv1.SessionStrategy_SESSION_STRATEGY_UNSPECIFIED))
			case "recent":
				runOpts = append(runOpts, client.WithSessionStrategy(runnerv1.SessionStrategy_SESSION_STRATEGY_MOST_RECENT))
			default:
				return fmt.Errorf("unknown session strategy %q", opts.SessionStrategy)
			}

			var runner client.Runner

			if opts.ServerAddr == "" {
				localRunner, err := client.NewLocalRunner(runOpts...)
				if err != nil {
					return err
				}

				runner = localRunner
			} else {
				remoteRunner, err := client.NewRemoteRunner(
					cmd.Context(),
					opts.ServerAddr,
					runOpts...,
				)
				if err != nil {
					return err
				}

				runner = remoteRunner
			}

			defer runner.Cleanup(cmd.Context())

			if opts.DryRun {
				return runner.DryRunBlock(ctx, block, cmd.ErrOrStderr())
			}

			err = runner.RunBlock(ctx, block)
			if err != nil {
				if err != nil && errors.Is(err, io.ErrClosedPipe) {
					err = nil
				}
			}
			return err
		},
	}

	setDefaultFlags(&cmd)

	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Print the final command without executing.")
	cmd.Flags().StringArrayVarP(&opts.ReplaceScripts, "replace", "r", nil, "Replace instructions using sed.")

	cmd.Flags().StringVarP(&opts.ServerAddr, "server", "s", os.Getenv("RUNME_SERVER_ADDR"), "Server address to connect runner to")
	cmd.Flags().StringVar(&opts.SessionID, "session", os.Getenv("RUNME_SESSION"), "Session id to run commands in runner inside of")

	cmd.Flags().StringVar(&opts.SessionStrategy, "session-strategy", func() string {
		if val, ok := os.LookupEnv("RUNME_SESSION_STRATEGY"); ok {
			return val
		}

		return "manual"
	}(), "Strategy for session selection. Options are manual, recent. Defaults to manual")

	_ = cmd.Flags().MarkHidden("session")
	_ = cmd.Flags().MarkHidden("session-strategy")

	return &cmd
}

func ctxWithSigCancel(ctx context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ctx)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	go func() {
		<-sigs
		cancel()
	}()

	return ctx, cancel
}

func replace(scripts []string, lines []string) error {
	if len(scripts) == 0 {
		return nil
	}

	for _, script := range scripts {
		engine, err := sed.New(strings.NewReader(script))
		if err != nil {
			return errors.Wrapf(err, "failed to compile sed script %q", script)
		}

		for idx, line := range lines {
			var err error
			lines[idx], err = engine.RunString(line)
			if err != nil {
				return errors.Wrapf(err, "failed to run sed script %q on line %q", script, line)
			}
		}
	}

	return nil
}

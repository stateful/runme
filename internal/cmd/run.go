package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/containerd/console"
	"github.com/fatih/color"
	"github.com/pkg/errors"
	"github.com/rwtodd/Go.Sed/sed"
	"github.com/spf13/cobra"
	"github.com/stateful/runme/internal/document"
	"github.com/stateful/runme/internal/runner/client"
	"github.com/stateful/runme/internal/tui"
)

func runCmd() *cobra.Command {
	var (
		dryRun         bool
		runAll         bool
		parallel       bool
		replaceScripts []string
		serverAddr     string
		category       string
		getRunnerOpts  func() ([]client.RunnerOption, error)
	)

	cmd := cobra.Command{
		Use:               "run <commands>",
		Aliases:           []string{"exec"},
		Short:             "Run a selected command",
		Long:              "Run a selected command identified based on its unique parsed name.",
		Args:              cobra.ArbitraryArgs,
		ValidArgsFunction: validCmdNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			runMany := runAll || (category != "" && len(args) == 0)
			if !runMany && len(args) == 0 {
				return errors.New("must provide at least one command to run")
			}

			runBlocks := make([]*document.CodeBlock, 0)

			{
				blocks, err := getCodeBlocks()
				if err != nil {
					return err
				}

				if runMany {
					for _, block := range blocks {
						if !block.ExcludeFromRunAll() {
							if category != "" && category != block.Category() {
								continue
							}
							runBlocks = append(runBlocks, block)
						}
					}
					if len(runBlocks) == 0 {
						return errors.New("No tasks to execute with the category provided")
					}
				} else {
					for _, arg := range args {
						block, err := lookupCodeBlock(blocks, arg)
						if err != nil {
							return err
						}

						if err := replace(replaceScripts, block.Lines()); err != nil {
							return err
						}

						runBlocks = append(runBlocks, block)
					}
				}
			}

			if runMany {
				err := confirmExecution(cmd, len(runBlocks), parallel, category)
				if err != nil {
					return err
				}
			}

			ctx, cancel := ctxWithSigCancel(cmd.Context())
			defer cancel()

			var stdin io.Reader

			// Use pipe here so that it can be closed and the command can exit.
			// Without this approach, the command would hang on reading from stdin.
			r, w := io.Pipe()
			stdin = r
			go func() { _, _ = io.Copy(w, cmd.InOrStdin()) }()

			runnerOpts, err := getRunnerOpts()
			if err != nil {
				return err
			}

			runnerOpts = append(
				runnerOpts,
				client.WithinShellMaybe(),
				client.WithStdin(stdin),
				client.WithStdout(cmd.OutOrStdout()),
				client.WithStderr(cmd.ErrOrStderr()),
			)

			var runner client.Runner

			if serverAddr == "" {
				localRunner, err := client.NewLocalRunner(runnerOpts...)
				if err != nil {
					return err
				}

				runner = localRunner
			} else {
				remoteRunner, err := client.NewRemoteRunner(
					cmd.Context(),
					serverAddr,
					runnerOpts...,
				)
				if err != nil {
					return err
				}

				runner = remoteRunner
			}

			blockColor := color.New(color.Bold, color.FgYellow)
			playColor := color.New(color.BgHiBlue, color.Bold, color.FgWhite)
			textColor := color.New()
			successColor := color.New(color.FgGreen)
			failureColor := color.New(color.FgRed)

			infoMsgPrefix := playColor.Sprint(" ► ")

			multiRunner := client.MultiRunner{
				Runner: runner,
				PreRunMsg: func(blocks []*document.CodeBlock, parallel bool) string {
					blockNames := make([]string, len(blocks))
					for i, block := range blocks {
						blockNames[i] = block.Name()
						blockNames[i] = blockColor.Sprint(blockNames[i])
					}

					scriptRunText := "Running task"
					if runMany && parallel {
						scriptRunText = "Running"
						blockNames = []string{blockColor.Sprint("all tasks")}
						if category != "" {
							blockNames = []string{blockColor.Sprintf("tasks for category %s", category)}
						}
					}

					if len(blocks) > 1 && !runMany {
						scriptRunText += "s"
					}

					extraText := ""

					if parallel {
						extraText = " in parallel"
					}

					return fmt.Sprintf(
						"%s %s %s%s...\n",
						infoMsgPrefix,
						textColor.Sprint(scriptRunText),
						strings.Join(blockNames, ", "),
						textColor.Sprint(extraText),
					)
				},
				PostRunMsg: func(block *document.CodeBlock, exitCode uint) string {
					var statusIcon string

					if exitCode == 0 {
						statusIcon = successColor.Sprint("✓")
					} else {
						statusIcon = failureColor.Sprint("𐄂")
					}

					return textColor.Sprintf(
						"%s %s %s %s %s %v\n",
						infoMsgPrefix,
						statusIcon,
						textColor.Sprint("Task"),
						blockColor.Sprint(block.Name()),
						textColor.Sprint("exited with code"),
						exitCode,
					)
				},
			}

			if parallel {
				multiRunner.StdoutPrefix = fmt.Sprintf("[%s] ", blockColor.Sprintf("%%s"))
			}

			defer multiRunner.Cleanup(cmd.Context())

			if dryRun {
				return runner.DryRunBlock(ctx, runBlocks[0], cmd.ErrOrStderr())
			}

			err = inRawMode(func() error {
				if len(runBlocks) > 1 {
					return multiRunner.RunBlocks(ctx, runBlocks, parallel)
				}

				return runner.RunBlock(ctx, runBlocks[0])
			})

			if err != nil {
				if err != nil && errors.Is(err, io.ErrClosedPipe) {
					err = nil
				}
			}
			return err
		},
	}

	setDefaultFlags(&cmd)

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print the final command without executing.")
	cmd.Flags().StringArrayVarP(&replaceScripts, "replace", "r", nil, "Replace instructions using sed.")
	cmd.Flags().BoolVarP(&parallel, "parallel", "p", false, "Run tasks in parallel.")
	cmd.Flags().BoolVarP(&runAll, "all", "a", false, "Run all commands.")
	cmd.Flags().StringVarP(&category, "category", "c", "", "Run from a specific category.")

	getRunnerOpts = setRunnerFlags(&cmd, &serverAddr)

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

func inRawMode(cb func() error) error {
	if !isTerminal(os.Stdout.Fd()) {
		return cb()
	}

	current := console.Current()
	_ = current.SetRaw()

	err := cb()

	_ = current.Reset()

	return err
}

func confirmExecution(cmd *cobra.Command, numTasks int, parallel bool, category string) error {
	text := fmt.Sprintf("Run all %d tasks", numTasks)
	if category != "" {
		text = fmt.Sprintf("Run %d tasks for category %s", numTasks, category)
	}
	if parallel {
		text += " (in parallel)"
	}

	text += "?"

	model := tui.NewStandaloneQuestionModel(
		text,
		tui.MinimalKeyMap,
		tui.DefaultStyles,
	)
	finalModel, err := newProgram(cmd, model).Run()
	if err != nil {
		return errors.Wrap(err, "cli program failed")
	}
	confirm := finalModel.(tui.StandaloneQuestionModel).Confirmed()

	if !confirm {
		return errors.New("operation cancelled")
	}

	return nil
}

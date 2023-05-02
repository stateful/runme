package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path"
	"strings"
	"sync"
	"syscall"

	"github.com/containerd/console"
	"github.com/fatih/color"
	"github.com/pkg/errors"
	"github.com/rwtodd/Go.Sed/sed"
	"github.com/spf13/cobra"
	"github.com/stateful/runme/internal/document"
	"github.com/stateful/runme/internal/project"
	"github.com/stateful/runme/internal/runner/client"
)

var (
	dryRun         bool
	runInParallel  bool
	runSequential  bool
	onlyCommandIO  bool
	replaceScripts []string
	serverAddr     string
	getRunnerOpts  func() ([]client.RunnerOption, error)
)

func runCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:               "run [...blocks]",
		Aliases:           []string{"exec"},
		Short:             "Run a selected command",
		Long:              "Run a selected command identified based on its unique parsed name.",
		Args:              cobra.ArbitraryArgs,
		ValidArgsFunction: validCmdNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			p := project.New(fChdir)
			if !isInExperimentalMode() {
				filePath := path.Join(fChdir, fFileName)
				blocks, err := p.GetCodeBlocks(filePath[len(p.RootDir):], fAllowUnknown, fIgnoreNameless)
				if err != nil {
					return err
				}

				block, err := lookupCodeBlock(blocks, args[0])
				if err != nil {
					return err
				}

				err = runBlock(*cmd, *block)
				if err != nil {
					return err
				}
				return nil
			}

			inParallelMode := false
			blocks := os.Args[2:]
			runMap := make(map[string][]string)
			var parallelBlocks []*document.CodeBlock
			for i, blockID := range blocks {
				// switch to parallel mode
				if isParallelParam(blockID) {
					inParallelMode = true
					continue
				}

				// run all parallel blocks if
				// - we are in parallel mode and look at the last item of the list
				// - we encounter a sequential parameter
				if isSequentialParam(blockID) || (inParallelMode && i == len(blocks)-1) {
					file, block, err := p.LookUpCodeBlockByID(blockID)
					if err != nil {
						return err
					}

					if !isSequentialParam(blockID) {
						runMap[*file] = append(runMap[*file], blockID)
						parallelBlocks = append(parallelBlocks, block)
					}

					err = printUpdate(runMap)
					if err != nil {
						return err
					}

					err = runBlocks(*cmd, parallelBlocks)
					if err != nil {
						return err
					}
					parallelBlocks = nil
					runMap = make(map[string][]string)
				}

				// switch to sequential mode
				if isSequentialParam(blockID) {
					inParallelMode = false
					continue
				}

				// collect parallel task and move on
				if inParallelMode {
					file, block, err := p.LookUpCodeBlockByID(blockID)
					if err != nil {
						return err
					}

					runMap[*file] = append(runMap[*file], blockID)
					parallelBlocks = append(parallelBlocks, block)
					continue
				}

				// run sequential block
				file, block, err := p.LookUpCodeBlockByID(blockID)
				if err != nil {
					return err
				}

				runMap[*file] = append(runMap[*file], blockID)
				err = printUpdate(runMap)
				if err != nil {
					return err
				}

				runMap = make(map[string][]string)
				err = runBlock(*cmd, *block)
				if err != nil {
					return err
				}
			}

			return nil
		},
	}

	setDefaultFlags(&cmd)

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print the final command without executing.")
	cmd.Flags().StringArrayVarP(&replaceScripts, "replace", "r", nil, "Replace instructions using sed.")

	if isInExperimentalMode() {
		cmd.Flags().BoolVarP(&runInParallel, "parallel", "p", false, "Run commands in parallel. (experimental)")
		cmd.Flags().BoolVarP(&runSequential, "sequential", "s", true, "Run commands sequentially. (experimental)")
		cmd.Flags().BoolVarP(&onlyCommandIO, "onlyCommandOutput", "", false, "If set, Runme will only output command output. (experimental)")
	}

	getRunnerOpts = setRunnerFlags(&cmd, &serverAddr)

	return &cmd
}

func isSequentialParam(blockID string) bool {
	return blockID == "-s" || blockID == "--sequential"
}

func isParallelParam(blockID string) bool {
	return blockID == "-p" || blockID == "--parallel"
}

func printUpdate(runMap map[string][]string) error {
	runme := color.New(color.BgHiBlue, color.Bold, color.FgWhite)
	_, err := runme.Print(" ► ")
	if err != nil {
		return err
	}

	_, err = fmt.Print(" Run ")
	if err != nil {
		return err
	}

	blockIDStr := color.New(color.Bold, color.FgYellow)

	cnt := 0
	for file, blockIDs := range runMap {
		cnt++

		for i, blockID := range blockIDs {
			_, err := blockIDStr.Printf("\"%s\"", blockID)
			if err != nil {
				return err
			}

			if (i + 1) != len(blockIDs) {
				_, err := fmt.Print(", ")
				if err != nil {
					return err
				}
			}
		}

		_, err := fmt.Print(" from ")
		if err != nil {
			return err
		}
		filePath := color.New(color.Bold, color.FgGreen)
		_, err = filePath.Print(file)
		if err != nil {
			return err
		}

		if cnt < len(runMap) {
			_, err := fmt.Print(", ")
			if err != nil {
				return err
			}
		}
	}

	_, err = fmt.Println("")
	if err != nil {
		return err
	}

	return nil
}

func runBlock(cmd cobra.Command, block document.CodeBlock) error {
	if err := replace(replaceScripts, block.Lines()); err != nil {
		return err
	}

	ctx, cancel := ctxWithSigCancel(cmd.Context())
	defer cancel()

	var stdin io.Reader

	if isInExperimentalMode() && block.Interactive() {
		warn := color.New(color.Bold, color.FgYellow)
		_, err := warn.Print("Warning: ")
		if err != nil {
			return err
		}
		fmt.Println("Interactive mode was disabled as it is not supported with the experimental CLI")
	}

	if block.Interactive() && !isInExperimentalMode() {
		// Use pipe here so that it can be closed and the command can exit.
		// Without this approach, the command would hang on reading from stdin.
		r, w := io.Pipe()
		stdin = r
		go func() { _, _ = io.Copy(w, cmd.InOrStdin()) }()
	} else {
		stdin = bytes.NewReader(nil)
	}

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

	defer runner.Cleanup(cmd.Context())

	if dryRun {
		return runner.DryRunBlock(ctx, &block, cmd.ErrOrStderr())
	}

	err = inRawMode(func() error {
		return runner.RunBlock(ctx, &block)
	})

	if err != nil {
		if err != nil && errors.Is(err, io.ErrClosedPipe) {
			err = nil
		}
	}
	return err
}

func runBlocks(cmd cobra.Command, blocks []*document.CodeBlock) error {
	var wg sync.WaitGroup

	wgDone := make(chan bool)
	chErr := make(chan error)

	for _, block := range blocks {
		wg.Add(1)
		go func(b *document.CodeBlock) {
			defer wg.Done()
			err := runBlock(cmd, *b)
			if err != nil {
				chErr <- err
				return
			}
		}(block)
	}

	go func() {
		wg.Wait()
		close(wgDone)
	}()

	select {
	case <-wgDone:
		// carry on
		break
	case err := <-chErr:
		close(chErr)
		return err
	}

	return nil
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

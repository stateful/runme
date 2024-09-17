package beta

import (
	"context"
	"io"
	"os"

	"github.com/creack/pty"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/stateful/runme/v3/internal/command"
	"github.com/stateful/runme/v3/internal/config/autoconfig"
	rcontext "github.com/stateful/runme/v3/internal/runner/context"
	"github.com/stateful/runme/v3/internal/runnerv2client"
	"github.com/stateful/runme/v3/internal/session"
	runnerv2 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/runner/v2"
	"github.com/stateful/runme/v3/pkg/document"
	"github.com/stateful/runme/v3/pkg/project"
)

func runCmd(*commonFlags) *cobra.Command {
	var remote bool

	cmd := cobra.Command{
		Use:     "run [command1 command2 ...]",
		Aliases: []string{"exec"},
		Short:   "Run one or more commands.",
		Long: `Run commands by providing their names delimited by space.
The names are interpreted as glob patterns.

In the case of multiple commands, they are executed one-by-one in the order they appear in the document.

The --tag option additionally filters the list of tasks to execute by tag.`,
		Example: `Run all blocks starting with the "generate-" prefix:
  runme beta run "generate-*"

Run all blocks from the "setup" and "teardown" tags:
  runme beta run --tag=setup,teardown
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return autoconfig.InvokeForCommand(
				func(
					clientFactory autoconfig.ClientFactory,
					cmdFactory command.Factory,
					filters []project.Filter,
					logger *zap.Logger,
					proj *project.Project,
				) error {
					defer logger.Sync()

					tasks, err := project.LoadTasks(cmd.Context(), proj)
					if err != nil {
						return err
					}
					logger.Info("found tasks", zap.Int("count", len(tasks)))

					argsFilter, err := createProjectFilterFromPatterns(args)
					if err != nil {
						return err
					}

					filters = append(filters, argsFilter)

					tasks, err = project.FilterTasksByFn(tasks, filters...)
					if err != nil {
						return err
					}
					logger.Info("filtered tasks by filters", zap.Int("count", len(tasks)))

					if len(tasks) == 0 {
						_, err := cmd.ErrOrStderr().Write([]byte("no tasks to run\n"))
						return errors.WithStack(err)
					}

					ctx := cmd.Context()

					if remote {
						client, err := clientFactory()
						if err != nil {
							return err
						}

						sessionResp, err := client.CreateSession(
							ctx,
							&runnerv2.CreateSessionRequest{
								Project: &runnerv2.Project{
									Root:         proj.Root(),
									EnvLoadOrder: proj.EnvFilesReadOrder(),
								},
							},
						)
						if err != nil {
							return errors.WithMessage(err, "failed to create session")
						}

						opts := runnerv2client.ExecuteProgramOptions{
							SessionID:        sessionResp.GetSession().GetId(),
							Stdin:            io.NopCloser(cmd.InOrStdin()),
							Stdout:           cmd.OutOrStdout(),
							Stderr:           cmd.ErrOrStderr(),
							StoreStdoutInEnv: true,
						}
						if stdin, ok := cmd.InOrStdin().(*os.File); ok {
							size, err := pty.GetsizeFull(stdin)
							if err != nil {
								logger.Info("failed to get terminal size", zap.Error(err))
							} else {
								opts.Winsize = &runnerv2.Winsize{
									Rows: uint32(size.Rows),
									Cols: uint32(size.Cols),
									X:    uint32(size.X),
									Y:    uint32(size.Y),
								}
							}
						}

						for _, t := range tasks {
							err := runCodeBlockWithClient(ctx, client, t.CodeBlock, opts)
							if err != nil {
								return err
							}
						}
					} else {
						session, err := session.New(
							session.WithOwl(false),
							session.WithSeedEnv(nil),
						)
						if err != nil {
							return err
						}

						options := createCommandOptions(cmd, session)

						for _, t := range tasks {
							err := runCodeBlock(ctx, t.CodeBlock, cmdFactory, options)
							if err != nil {
								return err
							}
						}
					}

					return nil
				},
			)
		},
	}

	cmd.Flags().BoolVarP(&remote, "remote", "r", false, "Run commands on a remote server.")

	return &cmd
}

func createCommandOptions(
	cmd *cobra.Command,
	sess *session.Session,
) command.CommandOptions {
	return command.CommandOptions{
		Session: sess,
		Stdin:   cmd.InOrStdin(),
		Stdout:  cmd.OutOrStdout(),
		Stderr:  cmd.ErrOrStderr(),
	}
}

func createProgramConfigFromCodeBlock(block *document.CodeBlock) (*command.ProgramConfig, error) {
	// TODO(adamb): [command.Config] is generated exclusively from the [document.CodeBlock].
	// As we introduce some document- and block-related configs in runme.yaml (root but also nested),
	// this [Command.Config] should be further extended.
	//
	// The way to do it is to use [config.Loader] and calling [config.Loader.FindConfigChain] with
	// task's document path. It will produce all the configs that are relevant to the document.
	// Next, they should be merged into a single [config.Config] in a correct order, starting from
	// the last element of the returned config chain. Finally, [command.Config] should be updated.
	// This algorithm should be likely encapsulated in the [internal/config] and [internal/command]
	// packages.
	return command.NewProgramConfigFromCodeBlock(block)
}

func runCodeBlock(
	ctx context.Context,
	block *document.CodeBlock,
	factory command.Factory,
	options command.CommandOptions,
) error {
	cfg, err := createProgramConfigFromCodeBlock(block)
	if err != nil {
		return err
	}

	cfg.Mode = runnerv2.CommandMode_COMMAND_MODE_CLI

	execInfo := &rcontext.ExecutionInfo{
		KnownName: block.Name(),
		KnownID:   block.ID(),
	}
	ctx = rcontext.ContextWithExecutionInfo(ctx, execInfo)

	cmd, err := factory.Build(cfg, options)
	if err != nil {
		return err
	}
	if err := cmd.Start(ctx); err != nil {
		return err
	}
	return cmd.Wait(ctx)
}

func runCodeBlockWithClient(
	ctx context.Context,
	client *runnerv2client.Client,
	block *document.CodeBlock,
	opts runnerv2client.ExecuteProgramOptions,
) error {
	cfg, err := createProgramConfigFromCodeBlock(block, command.WithInteractiveLegacy())
	if err != nil {
		return err
	}
	return client.ExecuteProgram(ctx, cfg, opts)
}

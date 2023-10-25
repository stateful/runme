package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/containerd/console"
	"github.com/fatih/color"
	"github.com/pkg/errors"
	"github.com/rwtodd/Go.Sed/sed"
	"github.com/spf13/cobra"
	"github.com/stateful/runme/internal/document"
	"github.com/stateful/runme/internal/runner/client"
	"github.com/stateful/runme/internal/tui"
	"github.com/stateful/runme/pkg/project"
	"golang.org/x/exp/slices"
)

const (
	exportExtractRegex string = `(\n*)export (\w+=)(("[^"]*")|('[^']*')|[^;\n]+)`
)

type CommandExportExtractMatch struct {
	Key            string
	Value          string
	Match          string
	HasStringValue bool
	LineNumber     int
}

func runCmd() *cobra.Command {
	var (
		dryRun                bool
		runAll                bool
		skipPrompts           bool
		skipPromptsExplicitly bool
		parallel              bool
		replaceScripts        []string
		serverAddr            string
		category              string
		getRunnerOpts         func() ([]client.RunnerOption, error)
		runIndex              int
	)

	cmd := cobra.Command{
		Use:               "run <commands>",
		Aliases:           []string{"exec"},
		Short:             "Run a selected command",
		Long:              "Run a selected command identified based on its unique parsed name.",
		Args:              cobra.ArbitraryArgs,
		ValidArgsFunction: validCmdNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			runWithIndex := fFileMode && runIndex >= 0

			runMany := runAll || (category != "" && len(args) == 0)
			if !runMany && len(args) == 0 && !runWithIndex {
				return errors.New("must provide at least one command to run")
			}

			proj, err := getProject()
			if err != nil {
				return err
			}

			runBlocks := make([]project.FileCodeBlock, 0)

			{
			searchBlocks:
				loader, err := newProjectLoader(cmd)
				if err != nil {
					return err
				}

				blocks, err := loader.LoadTasks(proj, fAllowUnknown, fAllowUnnamed, true)
				if err != nil {
					return err
				}

				if len(blocks) > 0 && blocks[0].Frontmatter.SkipPrompts {
					skipPrompts = true
				}

				if runWithIndex {
					if runIndex >= len(blocks) {
						return fmt.Errorf("command index %v out of range", runIndex)
					}

					runBlocks = []project.FileCodeBlock{blocks[runIndex]}
				} else if runMany {
					for _, fileBlock := range blocks {
						block := fileBlock.Block

						if runAll && block.ExcludeFromRunAll() {
							continue
						}

						if category != "" {
							if block.ExcludeFromRunAll() {
								continue
							}

							if !slices.Contains(strings.Split(block.Category(), ","), category) {
								continue
							}
						}

						runBlocks = append(runBlocks, fileBlock)
					}

					if len(runBlocks) == 0 && !fAllowUnnamed {
						fAllowUnnamed = true
						goto searchBlocks
					}
				} else {
					for _, arg := range args {
						block, err := lookupCodeBlockWithPrompt(cmd, arg, blocks)
						if err != nil {
							if project.IsCodeBlockNotFoundError(err) && !fAllowUnnamed {
								fAllowUnnamed = true
								goto searchBlocks
							}

							return err
						}

						if err := replace(replaceScripts, block.Block.Lines()); err != nil {
							return err
						}

						runBlocks = append(runBlocks, block)
					}
				}
			}

			if len(runBlocks) == 0 {
				return errors.New("No tasks to execute with the category provided")
			}

			ctx, cancel := ctxWithSigCancel(cmd.Context())
			defer cancel()

			runnerOpts, err := getRunnerOpts()
			if err != nil {
				return err
			}

			runnerOpts = append(
				runnerOpts,
				client.WithinShellMaybe(),
				client.WithStdin(cmd.InOrStdin()),
				client.WithStdout(cmd.OutOrStdout()),
				client.WithStderr(cmd.ErrOrStderr()),
				client.WithProject(proj),
			)

			preRunOpts := []client.RunnerOption{
				client.WrapWithCancelReader(),
			}

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

			sessionEnvs, err := runner.GetEnvs(ctx)
			if err != nil {
				return err
			}
			if (skipPromptsExplicitly || isTerminal(os.Stdout.Fd())) && !skipPrompts {
				err = promptEnvVars(cmd, sessionEnvs, runBlocks...)
				if err != nil {
					return err
				}

				if runMany {
					err := confirmExecution(cmd, len(runBlocks), parallel, category)
					if err != nil {
						return err
					}
				}
			}

			blockColor := color.New(color.Bold, color.FgYellow)
			playColor := color.New(color.BgHiBlue, color.Bold, color.FgWhite)
			textColor := color.New()
			successColor := color.New(color.FgGreen)
			failureColor := color.New(color.FgRed)

			infoMsgPrefix := playColor.Sprint(" ► ")

			multiRunner := client.MultiRunner{
				Runner: runner,
				PreRunMsg: func(blocks []project.FileCodeBlock, parallel bool) string {
					blockNames := make([]string, len(blocks))
					for i, block := range blocks {
						blockNames[i] = block.GetBlock().Name()
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
				PreRunOpts: preRunOpts,
			}

			if parallel {
				multiRunner.StdoutPrefix = fmt.Sprintf("[%s] ", blockColor.Sprintf("%%s"))
			}

			defer multiRunner.Cleanup(cmd.Context())

			if dryRun {
				return runner.DryRunBlock(ctx, runBlocks[0], cmd.ErrOrStderr()) // #nosec G602; runBlocks is checked
			}

			err = inRawMode(func() error {
				if len(runBlocks) > 1 {
					return multiRunner.RunBlocks(ctx, runBlocks, parallel)
				}

				if err := client.ApplyOptions(runner, preRunOpts...); err != nil {
					return err
				}

				return runner.RunBlock(ctx, runBlocks[0]) // #nosec G602; runBlocks comes from the parent scope and is checked
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
	cmd.Flags().BoolVar(&skipPrompts, "skip-prompts", false, "Skip prompting for variables.")
	cmd.Flags().StringVarP(&category, "category", "c", "", "Run from a specific category.")
	cmd.Flags().IntVarP(&runIndex, "index", "i", -1, "Index of command to run, 0-based. (Ignored in project mode)")
	cmd.PreRun = func(cmd *cobra.Command, args []string) {
		skipPromptsExplicitly = cmd.Flags().Changed("skip-prompts")
	}

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

const fileNameSeparator = "/"

func splitRunArgument(name string) (queryFile string, queryName string, err error) {
	parts := strings.SplitN(name, fileNameSeparator, 2)

	if len(parts) > 1 {
		queryFile = parts[0]
		queryName = parts[1]
	} else {
		queryName = name
	}

	return
}

var (
	blockPromptListItemStyle = lipgloss.NewStyle().PaddingLeft(0).Bold(true)
	blockPromptAppStyle      = lipgloss.NewStyle().Margin(1, 2)
)

type blockPromptItem struct {
	block *project.CodeBlock
}

func (i blockPromptItem) FilterValue() string {
	return i.block.Block.Name()
}

func (i blockPromptItem) Title() string {
	return blockPromptListItemStyle.Render(i.block.Block.Name())
}

func (i blockPromptItem) Description() string {
	return i.block.File
}

type RunBlockPrompt struct {
	list.Model
	selectedBlock list.Item
	heading       string
}

func (p RunBlockPrompt) Init() tea.Cmd {
	return nil
}

func (p RunBlockPrompt) KeyMap() *tui.KeyMap {
	kmap := tui.NewKeyMap()

	kmap.Set("enter", key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	))

	return kmap
}

func (p RunBlockPrompt) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h, v := blockPromptAppStyle.GetFrameSize()
		p.SetSize(msg.Width-h, msg.Height-v-len(strings.Split(p.heading, "\n")))
	case tea.KeyMsg:
		kmap := p.KeyMap()

		if kmap.Matches(msg, "enter") {
			p.selectedBlock = p.SelectedItem()
			return p, tea.Quit
		}
	}

	model, cmd := p.Model.Update(msg)
	p.Model = model

	return p, cmd
}

func (p RunBlockPrompt) View() string {
	content := ""

	content += p.heading
	content += p.Model.View()

	return blockPromptAppStyle.Render(content)
}

func lookupCodeBlockWithPrompt(cmd *cobra.Command, query string, srcBlocks project.CodeBlocks) (*project.CodeBlock, error) {
	queryFile, queryName, err := splitRunArgument(query)
	if err != nil {
		return nil, err
	}

	blocks, err := srcBlocks.LookupWithFile(queryFile, queryName)
	if err != nil {
		return nil, err
	}

	var block project.CodeBlock

	if len(blocks) > 1 {
		if !isTerminal(os.Stdout.Fd()) {
			return nil, fmt.Errorf("multiple matches found for code block; please use a file specifier in the form \"{file}%s{task-name}\"", fileNameSeparator)
		}

		pBlock, err := promptForRun(cmd, blocks)
		if err != nil {
			return nil, err
		}
		block = *pBlock
	} else {
		block = blocks[0]
	}

	return &block, nil
}

func promptForRun(cmd *cobra.Command, blocks project.CodeBlocks) (*project.CodeBlock, error) {
	items := make([]list.Item, len(blocks))

	for i := range blocks {
		items[i] = blockPromptItem{
			block: &blocks[i],
		}
	}

	l := list.New(
		items,
		list.NewDefaultDelegate(),
		0,
		0,
	)

	l.SetFilteringEnabled(false)
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)
	l.SetShowTitle(false)

	l.Title = "Select Task"

	heading := fmt.Sprintf("Found multiple matching tasks. Select from the following.\n\nNote that you can avoid this screen by providing a filename specifier, such as \"{filename}%s{task}\"\n\n\n", fileNameSeparator)

	model := RunBlockPrompt{
		Model:   l,
		heading: heading,
	}

	prog := newProgramWithOutputs(cmd.OutOrStdout(), cmd.InOrStdin(), model, tea.WithAltScreen())
	m, err := prog.Run()
	if err != nil {
		return nil, err
	}

	result := m.(RunBlockPrompt).selectedBlock

	if result == nil {
		return nil, errors.New("no block selected")
	}

	return result.(blockPromptItem).block, nil
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

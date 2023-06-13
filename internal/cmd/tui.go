package cmd

import (
	"fmt"
	"math"
	"strings"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	fuzzyfinder "github.com/ktr0731/go-fuzzyfinder"
	"github.com/mgutz/ansi"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	rmath "github.com/stateful/runme/internal/math"
	"github.com/stateful/runme/internal/project"
	"github.com/stateful/runme/internal/runner"
	"github.com/stateful/runme/internal/runner/client"
	"github.com/stateful/runme/internal/version"
)

func tuiCmd() *cobra.Command {
	var (
		visibleEntries int
		runOnce        bool
		serverAddr     string
		filter         string
		getRunnerOpts  func() ([]client.RunnerOption, error)
	)

	cmd := cobra.Command{
		Use:   "tui",
		Short: "Run the interactive TUI",
		Long:  "Run a command from a descriptive list given by an interactive TUI.",
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, err := getProject()
			if err != nil {
				return err
			}

			blocks, err := proj.LoadTasks()
			if err != nil {
				return err
			}

			blocks = sortBlocks(blocks)

			if len(blocks) == 0 {
				return errors.Errorf("no code blocks in %s", fFileName)
			}

			if visibleEntries <= 0 {
				visibleEntries = math.MaxInt32
			}

			var runnerClient client.Runner

			defer func() {
				if runnerClient != nil {
					_ = runnerClient.Cleanup(cmd.Context())
				}
			}()

			runnerOpts, err := getRunnerOpts()
			if err != nil {
				return err
			}

			runnerOpts = append(
				runnerOpts,
				client.WithStdin(cmd.InOrStdin()),
				client.WithStdout(cmd.OutOrStdout()),
				client.WithStderr(cmd.ErrOrStderr()),
				client.WithProject(proj),
			)

			if serverAddr != "" {
				remoteRunner, err := client.NewRemoteRunner(
					cmd.Context(),
					serverAddr,
					runnerOpts...,
				)
				if err != nil {
					return errors.Wrap(err, "failed to create remote runner")
				}

				runnerClient = remoteRunner
			} else {
				localRunner, err := client.NewLocalRunner(
					runnerOpts...,
				)
				if err != nil {
					return errors.Wrap(err, "failed to create local runner")
				}

				runnerClient = localRunner
			}

			model := tuiModel{
				blocks: blocks,
				header: fmt.Sprintf(
					"%s %s\n\n",
					ansi.Color("runme", "57+b"),
					ansi.Color(version.BuildVersion, "white+d"),
				),
				visibleEntries: visibleEntries,
				expanded:       make(map[int]struct{}),
			}

			for {
				prog := newProgramWithOutputs(nil, cmd.InOrStdin(), model)

				newModel, err := prog.Run()
				if err != nil {
					return errors.WithStack(err)
				}

				model = newModel.(tuiModel)
				result := model.result

				if result.block == nil {
					break
				}

				ctx, cancel := ctxWithSigCancel(cmd.Context())

				runBlock := result.block.Clone()

				err = promptEnvVars(cmd, runBlock)
				if err != nil {
					return err
				}

				err = inRawMode(func() error {
					return client.WithTempSettings(
						runnerClient,
						[]client.RunnerOption{
							client.WrapWithCancelReader(),
						},
						func() error {
							return runnerClient.RunBlock(ctx, runBlock)
						},
					)
				})

				cancel()

				if err != nil {
					var eerror *runner.ExitError
					if !errors.As(err, &eerror) {
						return err
					}
					cmd.Printf(ansi.Color("%s", "red")+"\n", eerror)
				}

				if runOnce || result.exit {
					break
				}

				cmd.Print("\n")

				model.moveCursor(1)
			}

			return nil
		},
	}

	setDefaultFlags(&cmd)

	cmd.Flags().BoolVar(&runOnce, "exit", false, "Exit TUI after running a command")
	cmd.Flags().IntVar(&visibleEntries, "entries", defaultVisibleEntries, "Number of entries to show in TUI")
	cmd.Flags().StringVar(&filter, "filter", "", "Regular expression to filter results, by filename and task name")

	getRunnerOpts = setRunnerFlags(&cmd, &serverAddr)

	return &cmd
}

type tuiModel struct {
	blocks         project.CodeBlocks
	header         string
	visibleEntries int
	expanded       map[int]struct{}
	cursor         int
	scroll         int
	result         tuiResult
}

type tuiResult struct {
	block *project.CodeBlock
	exit  bool
}

func (m *tuiModel) numBlocksShown() int {
	return rmath.Min(len(m.blocks), m.visibleEntries)
}

func (m *tuiModel) maxScroll() int {
	return len(m.blocks) - m.numBlocksShown()
}

func (m *tuiModel) scrollBy(delta int) {
	m.scroll = rmath.Clamp(
		m.scroll+delta,
		0, m.maxScroll(),
	)
}

func (m *tuiModel) moveCursor(delta int) {
	m.cursor = rmath.Clamp(
		m.cursor+delta,
		0, len(m.blocks)-1,
	)

	if m.cursor < m.scroll || m.cursor >= m.scroll+m.numBlocksShown() {
		m.scrollBy(delta)
	}
}

func (m tuiModel) Init() tea.Cmd {
	return nil
}

const (
	tab                   = "  "
	defaultVisibleEntries = 5
)

func (m tuiModel) View() string {
	var s strings.Builder

	_, _ = s.WriteString(m.header)

	for i := m.scroll; i < m.scroll+m.numBlocksShown(); i++ {
		fileBlock := m.blocks[i]
		block := fileBlock.Block

		active := i == m.cursor
		_, expanded := m.expanded[i]

		line := " "
		if active {
			line = ">"
		}

		line += " "

		{
			name := block.Name()
			filename := ansi.Color(fileBlock.File, "white+d")

			if active {
				name = ansi.Color(name, "white+b")
			}

			identifier := fmt.Sprintf(
				"%s %s",
				name,
				filename,
			)

			line += identifier + "\n"
		}

		codeLines := block.Lines()

		for i, codeLine := range codeLines {
			content := tab + tab + codeLine

			if !expanded && len(codeLines) > 1 {
				content += " (...)"
			}

			content = ansi.Color(content, "white+d")

			if i >= 1 && !expanded {
				break
			}

			line += content + "\n\n"
		}

		_, _ = s.WriteString(line)
	}

	_, _ = s.WriteRune('\n')

	{
		help := strings.Join(
			[]string{
				fmt.Sprintf("%d/%d", m.cursor+1, len(m.blocks)),
				"Choose ↑↓←→",
				"Run [Enter]",
				"Search [f, crl+f]",
				"Expand [Space]",
				"Quit [q]",
			},
			tab,
		)

		help = ansi.Color(help, "white+d")

		_, _ = s.WriteString(help)
	}

	return s.String()
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, isKeyPress := msg.(tea.KeyMsg)

	if isKeyPress {
		switch keyMsg.String() {
		case "ctrl+c", "q":
			m.result = tuiResult{
				exit: true,
			}

			return m, tea.Quit

		case "up", "k":
			m.moveCursor(-1)

		case "down", "j":
			m.moveCursor(1)

		case " ":
			if _, ok := m.expanded[m.cursor]; ok {
				delete(m.expanded, m.cursor)
			} else {
				m.expanded[m.cursor] = struct{}{}
			}

		case "y", "c":
			command := strings.Join(m.blocks[m.cursor].Block.Lines(), "\n")
			_ = clipboard.WriteAll(command)

		case "enter", "l":
			m.result = tuiResult{
				block: &m.blocks[m.cursor],
			}

			return m, tea.Quit

		case "ctrl+f", "f":
			filteredNumbers, err := filterBlocks(m.blocks)
			if err != nil {
				return m, tea.Quit
			}

			m.blocks = filteredNumbers
		}
	}

	return m, nil
}

func filterBlocks(bs project.CodeBlocks) (fb project.CodeBlocks, e error) {
	indexes, err := fuzzyfinder.FindMulti(
		bs,
		func(i int) string {
			return bs[i].GetBlock().Name()
		},
		fuzzyfinder.WithPreviewWindow(func(i, w, h int) string {
			codeLines := bs[i].GetBlock().Lines()
			if i == -1 {
				return ""
			}

			lines := ""
			for _, codeLine := range codeLines {
				content := codeLine
				content = ansi.Color(content, "white+d")

				lines += content + "\n\n"
			}
			return fmt.Sprintf("--- %s ---\n\n%s", ansi.Color(bs[i].File, "white+d"), lines)
		}))
	filteredNumbers := project.CodeBlocks{}
	// Iterate through the indexes
	for _, index := range indexes {
		// Check if the index is within the array bounds
		if index >= 0 && index < len(bs) {
			filteredNumbers = append(filteredNumbers, bs[index])
		}
	}
	return filteredNumbers, err
}

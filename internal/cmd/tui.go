package cmd

import (
	"fmt"
	"math"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mgutz/ansi"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/stateful/runme/internal/document"
	rmath "github.com/stateful/runme/internal/math"
	"github.com/stateful/runme/internal/runner"
	"github.com/stateful/runme/internal/runner/client"
	"github.com/stateful/runme/internal/version"
)

func tuiCmd() *cobra.Command {
	var (
		visibleEntries int
		runOnce        bool
		serverAddr     string
	)

	cmd := cobra.Command{
		Use:   "tui",
		Short: "Run the interactive TUI",
		Long:  "Run a command from a descriptive list given by an interactive TUI.",
		RunE: func(cmd *cobra.Command, args []string) error {
			blocks, err := getCodeBlocks()
			if err != nil {
				return err
			}

			if len(blocks) == 0 {
				return errors.Errorf("no code blocks in %s", fFileName)
			}

			if visibleEntries <= 0 {
				visibleEntries = math.MaxInt32
			}

			var runnerClient client.Runner

			defer func() { _ = runnerClient.Cleanup(cmd.Context()) }()

			opts := []client.RunnerOption{
				client.WithDir(fChdir),
				client.WithStdin(cmd.InOrStdin()),
				client.WithStdout(cmd.OutOrStdout()),
				client.WithStderr(cmd.ErrOrStderr()),
			}

			if serverAddr != "" {
				remoteRunner, err := client.NewRemoteRunner(
					cmd.Context(),
					serverAddr,
					opts...,
				)

				if err != nil {
					return errors.Wrap(err, "failed to create remote runner")
				}

				runnerClient = remoteRunner
			} else {
				localRunner, err := client.NewLocalRunner(
					opts...,
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
				prog := newProgram(cmd, model)

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

				err = runnerClient.RunBlock(ctx, result.block)

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
	cmd.Flags().StringVar(&serverAddr, "server", "", "Server address to conenct TUI to")

	return &cmd
}

type tuiModel struct {
	blocks         document.CodeBlocks
	header         string
	visibleEntries int
	expanded       map[int]struct{}
	cursor         int
	scroll         int
	result         tuiResult
}

type tuiResult struct {
	block *document.CodeBlock
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
		block := m.blocks[i]

		active := i == m.cursor
		_, expanded := m.expanded[i]

		line := " "
		if active {
			line = ">"
		}

		line += " "

		{
			name := block.Name()
			lang := ansi.Color(block.Language(), "white+d")

			if active {
				name = ansi.Color(name, "white+b")
			} else {
				lang = ""
			}

			identifier := fmt.Sprintf(
				"%s %s",
				name,
				lang,
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

			line += content + "\n"
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
				"Expand [Space]",
				"Quit [q]",
				"  by Stateful",
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

		case "enter", "l":
			m.result = tuiResult{
				block: m.blocks[m.cursor],
			}

			return m, tea.Quit
		}
	}

	return m, nil
}

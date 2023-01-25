package cmd

import (
	"fmt"
	goMath "math"
	"runtime/debug"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mgutz/ansi"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/stateful/runme/internal/document"
	"github.com/stateful/runme/internal/math"
)

type tuiModel struct {
	blocks     document.CodeBlocks
	expanded   map[int]struct{}
	version    string
	numEntries int
	cursor     int
	scroll     int
	result     tuiResult
}

type tuiResult struct {
	run  *document.CodeBlock
	exit bool
}

func (m *tuiModel) numBlocksShown() int {
	return math.Min(len(m.blocks), m.numEntries)
}

func (m *tuiModel) maxScroll() int {
	return len(m.blocks) - m.numBlocksShown()
}

func (m *tuiModel) scrollBy(delta int) {
	m.scroll = math.Clamp(
		m.scroll+delta,
		0, m.maxScroll(),
	)
}

func (m *tuiModel) moveCursor(delta int) {
	m.cursor = math.Clamp(
		m.cursor+delta,
		0, len(m.blocks)-1,
	)

	if m.cursor < m.scroll || m.cursor >= m.scroll+m.numBlocksShown() {
		m.scrollBy(delta)
	}
}

const (
	tab               = "  "
	defaultNumEntries = 5
)

func (m tuiModel) View() string {
	s := fmt.Sprintf(
		"%s %s",
		ansi.Color("runme", "57+b"),
		ansi.Color(m.version, "white+d"),
	)

	s += "\n\n"

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

		s += line
	}

	s += "\n"

	{
		help := strings.Join(
			[]string{
				fmt.Sprintf("%v/%v", m.cursor+1, len(m.blocks)),
				"Choose ↑↓←→",
				"Run [Enter]",
				"Expand [Space]",
				"Quit [q]",
				"  by Stateful",
			},
			tab,
		)

		help = ansi.Color(help, "white+d")

		s += help
	}

	return s
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
				run: m.blocks[m.cursor],
			}

			return m, tea.Quit
		}
	}

	return m, nil
}

func tuiCmd() *cobra.Command {
	numEntries := 5
	exitAfterRun := false

	cmd := cobra.Command{
		Use:   "tui",
		Short: "Run the interactive TUI",
		Long:  "Run a command from a descriptive list given by an interactive TUI",
		RunE: func(cmd *cobra.Command, args []string) error {
			blocks, err := getCodeBlocks()
			if err != nil {
				return err
			}

			if len(blocks) == 0 {
				return errors.Errorf("no scripts in %s", fFileName)
			}

			version := "???"

			bi, ok := debug.ReadBuildInfo()
			if ok {
				version = bi.Main.Version
			}

			if numEntries <= 0 {
				numEntries = goMath.MaxInt32
			}

			model := tuiModel{
				blocks:     blocks,
				version:    version,
				expanded:   make(map[int]struct{}),
				numEntries: numEntries,
			}

			for {
				prog := tea.NewProgram(model)

				newModel, err := prog.Run()
				if err != nil {
					return err
				}

				model = newModel.(tuiModel)
				result := model.result

				if result.run == nil {
					break
				}

				if err = runBlock(cmd, result.run, nil); err != nil {
					if _, err := fmt.Printf(ansi.Color("%v", "red")+"\n", err); err != nil {
						return err
					}
				}

				if exitAfterRun || result.exit {
					break
				}

				if _, err := fmt.Print("\n"); err != nil {
					return err
				}

				model.moveCursor(1)
			}

			return nil
		},
	}

	setDefaultFlags(&cmd)

	cmd.Flags().BoolVar(&exitAfterRun, "exit", false, "Exit runme TUI after running a command")
	cmd.Flags().IntVar(&numEntries, "entries", defaultNumEntries, "Number of entries to show in TUI")

	return &cmd
}

func (m tuiModel) Init() tea.Cmd {
	return nil
}

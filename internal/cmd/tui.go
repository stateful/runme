package cmd

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mgutz/ansi"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/stateful/runme/v3/internal/runner"
	"github.com/stateful/runme/v3/internal/runner/client"
	"github.com/stateful/runme/v3/internal/version"
	"github.com/stateful/runme/v3/pkg/project"
	"golang.org/x/exp/constraints"
)

func tuiCmd() *cobra.Command {
	var (
		visibleEntries int
		runOnce        bool
		serverAddr     string
		getRunnerOpts  func() ([]client.RunnerOption, error)
	)

	cmd := cobra.Command{
		Use:   "tui",
		Short: "Run the interactive TUI",
		Long:  "Run a command from a descriptive list given by an interactive TUI.",
		RunE: func(cmd *cobra.Command, args []string) error {
			tasks, err := getAllProjectTasks(cmd)
			if err != nil {
				return err
			}

			defaultAllowUnnamed := fAllowUnnamed

			if !defaultAllowUnnamed {
				found := false

				for _, task := range tasks {
					if !fAllowUnknown && task.CodeBlock.IsUnknown() {
						continue
					}

					if !fAllowUnnamed && task.CodeBlock.IsUnnamed() {
						continue
					}

					found = true

					break
				}

				if !found {
					defaultAllowUnnamed = true
				}
			}

			if len(tasks) == 0 {
				if fFileMode {
					return errors.Errorf("no code blocks in %s", fFileName)
				}
				if !fAllowUnnamed {
					return errors.Errorf("no named code blocks, consider adding flag --allow-unnamed")
				}
				return errors.Errorf("no code blocks")
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

			proj, err := getProject()
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

			runnerClient, err = client.New(cmd.Context(), serverAddr, fSkipRunnerFallback, runnerOpts)
			if err != nil {
				return errors.Wrap(err, "failed to create local runner")
			}

			model := tuiModel{
				unfilteredTasks: tasks,
				header: fmt.Sprintf(
					"%s %s\n\n",
					ansi.Color("runme", "57+b"),
					ansi.Color(version.BuildVersion, "white+d"),
				),
				visibleEntries: visibleEntries,
				expanded:       make(map[int]struct{}),

				allowUnnamed: defaultAllowUnnamed,
				allowUnknown: fAllowUnknown,
			}

			model.filterCodeBlocks()

			for {
				prog := newProgramWithOutputs(nil, cmd.InOrStdin(), model)

				newModel, err := prog.Run()
				if err != nil {
					return errors.WithStack(err)
				}

				model = newModel.(tuiModel)
				result := model.result

				if result.task == (project.Task{}) {
					break
				}

				ctx, cancel := ctxWithSigCancel(cmd.Context())

				task := result.task

				doc := task.CodeBlock.Document()
				fmtr, err := doc.FrontmatterWithError()
				if err != nil {
					return err
				}

				if fmtr != nil && !fmtr.SkipPrompts {
					err = promptEnvVars(cmd, runnerClient, task)
					if err != nil {
						return err
					}
				}

				err = inRawMode(func() error {
					return client.WithTempSettings(
						runnerClient,
						[]client.RunnerOption{
							client.WrapWithCancelReader(),
						},
						func() error {
							return runnerClient.RunTask(ctx, task)
						},
					)
				})

				cancel()

				exitCode := uint(0)
				if err != nil {
					var eerror *runner.ExitError
					if !errors.As(err, &eerror) {
						return err
					}
					exitCode = eerror.Code
					cmd.Printf(ansi.Color("%s", "red")+"\n", eerror)
				}

				if runOnce || result.exit {
					break
				}

				cmd.Print("\n")

				if exitCode == 0 {
					model.moveCursor(1)
				}
			}

			return nil
		},
	}

	setDefaultFlags(&cmd)

	cmd.Flags().BoolVar(&runOnce, "exit", false, "Exit TUI after running a command")
	cmd.Flags().IntVar(&visibleEntries, "entries", defaultVisibleEntries, "Number of entries to show in TUI")

	getRunnerOpts = setRunnerFlags(&cmd, &serverAddr)

	return &cmd
}

type tuiModel struct {
	unfilteredTasks []project.Task
	tasks           []project.Task
	header          string
	visibleEntries  int
	expanded        map[int]struct{}
	cursor          int
	scroll          int
	result          tuiResult
	allowUnnamed    bool
	allowUnknown    bool
}

type tuiResult struct {
	task project.Task
	exit bool
}

func (m *tuiModel) numBlocksShown() int {
	return min(len(m.tasks), m.visibleEntries)
}

func (m *tuiModel) maxScroll() int {
	return len(m.tasks) - m.numBlocksShown()
}

func (m *tuiModel) scrollBy(delta int) {
	m.scroll = clamp(
		m.scroll+delta,
		0, m.maxScroll(),
	)
}

func (m *tuiModel) filterCodeBlocks() {
	hasInitialized := m.tasks != nil

	var oldSelection project.Task
	if hasInitialized {
		oldSelection = m.tasks[m.cursor]
	}

	m.tasks, _ = project.FilterTasksByFn(m.unfilteredTasks, func(t project.Task) (bool, error) {
		if !m.allowUnknown && t.CodeBlock.IsUnknown() {
			return false, nil
		}

		if !m.allowUnnamed && t.CodeBlock.IsUnnamed() {
			return false, nil
		}

		return true, nil
	})

	if !hasInitialized {
		return
	}

	foundOldSelection := false
	for i, task := range m.tasks {
		if task == oldSelection {
			m.moveCursorTo(i)
			foundOldSelection = true
			break
		}
	}

	if !foundOldSelection {
		if m.cursor >= len(m.tasks) {
			m.moveCursorTo(len(m.tasks) - 1)
		}
	}
}

func (m *tuiModel) moveCursorTo(newPos int) {
	m.moveCursor(newPos - m.cursor)
}

func (m *tuiModel) moveCursor(delta int) {
	m.cursor = max(0, clamp(
		m.cursor+delta,
		0, len(m.tasks)-1,
	))

	if m.cursor < m.scroll || m.cursor >= m.scroll+m.numBlocksShown() {
		m.scrollBy(delta)
	}
}

func (m tuiModel) Init() tea.Cmd {
	return nil
}

const (
	tab                   = "  "
	defaultVisibleEntries = 6
)

func (m tuiModel) View() string {
	var s strings.Builder

	_, _ = s.WriteString(m.header)

	for i := m.scroll; i < m.scroll+m.numBlocksShown(); i++ {
		task := m.tasks[i]
		block := task.CodeBlock

		active := i == m.cursor
		_, expanded := m.expanded[i]

		line := " "
		if active {
			line = ">"
		}

		line += " "

		{
			name := block.Name()
			if block.IsUnnamed() {
				name += " (unnamed)"
			}

			relFilename := project.GetRelativePath(getCwd(), task.DocumentPath)
			filename := ansi.Color(relFilename, "white+d")

			if active {
				name = ansi.Color(name, "white+ub")
			}

			intro := block.Intro()
			words := strings.Split(intro, " ")
			// todo(sebastian): this likely only works well for English
			max := 9 - len(strings.Split(relFilename, string(filepath.Separator)))
			if len(words) > max {
				intro = strings.Join(words[:max], " ") + "..."
			}

			if len(intro) > 0 {
				intro = ": " + intro
			}

			identifier := fmt.Sprintf(
				"%s %s%s",
				name,
				filename,
				intro,
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

	var unnamedVerb string
	if m.allowUnnamed {
		unnamedVerb = "Hide"
	} else {
		unnamedVerb = "Show"
	}

	{
		help := strings.Join(
			[]string{
				fmt.Sprintf("%d/%d", m.cursor+1, len(m.tasks)),
				"Choose ↑↓←→",
				"Run [Enter]",
				"Expand [Space]",
				fmt.Sprintf("%s Unnamed [u]", unnamedVerb),
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
			command := strings.Join(m.tasks[m.cursor].CodeBlock.Lines(), "\n")
			_ = clipboard.WriteAll(command)

		case "enter", "l":
			m.result = tuiResult{
				task: m.tasks[m.cursor],
			}

			return m, tea.Quit

		case "u":
			m.allowUnnamed = !m.allowUnnamed
			m.filterCodeBlocks()
		}
	}

	return m, nil
}

func clamp[T constraints.Ordered](x, a, b T) T {
	return min(b, max(a, x))
}

package project

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkg/errors"
)

type ProjectLoader struct {
	w          io.Writer
	r          io.Reader
	isTerminal bool
}

func NewLoader(w io.Writer, r io.Reader, isTerminal bool) ProjectLoader {
	return ProjectLoader{
		w:          w,
		r:          r,
		isTerminal: isTerminal,
	}
}

type loadTasksModel struct {
	spinner spinner.Model

	status   string
	filename string

	clear bool

	err error

	tasks CodeBlocks
	files []string

	nextTaskMsg tea.Cmd
}

type loadTaskFinished struct{}

func (pl ProjectLoader) newLoadTasksModel(nextTaskMsg tea.Cmd) loadTasksModel {
	return loadTasksModel{
		spinner:     spinner.New(spinner.WithSpinner(spinner.MiniDot)),
		nextTaskMsg: nextTaskMsg,
		status:      "Initializing...",
		tasks:       make(CodeBlocks, 0),
	}
}

func (pl ProjectLoader) LoadFiles(proj Project) ([]string, error) {
	m, err := pl.runTasksModel(proj, true)
	if err != nil {
		return nil, err
	}

	return m.files, nil
}

func (pl ProjectLoader) LoadTasks(proj Project, allowUnknown bool, allowUnnamed bool, filter bool) (CodeBlocks, error) {
	m, err := pl.runTasksModel(proj, false)
	if err != nil {
		return nil, err
	}

	tasks := m.tasks

	if filter {
		tasks = FilterCodeBlocks[CodeBlock](m.tasks, allowUnknown, allowUnnamed)

		if len(tasks) == 0 {
			// try again without filtering unnamed
			tasks = FilterCodeBlocks[CodeBlock](m.tasks, allowUnknown, true)
		}
	}

	return tasks, nil
}

func (pl ProjectLoader) runTasksModel(proj Project, filesOnly bool) (*loadTasksModel, error) {
	channel := make(chan interface{})
	go proj.LoadTasks(filesOnly, channel)

	nextTaskMsg := func() tea.Msg {
		msg, ok := <-channel

		if !ok {
			return loadTaskFinished{}
		}

		return msg
	}

	m := pl.newLoadTasksModel(nextTaskMsg)

	resultModel := m

	if pl.isTerminal {
		p := tea.NewProgram(m, tea.WithOutput(pl.w), tea.WithInput(pl.r))
		result, err := p.Run()
		if err != nil {
			return nil, err
		}

		resultModel = result.(loadTasksModel)
	} else {
		if strings.ToLower(os.Getenv("RUNME_VERBOSE")) != "true" {
			pl.w = io.Discard
		}

		_, _ = fmt.Fprintln(pl.w, "Initializing...")

	outer:
		for {
			if resultModel.err != nil {
				break
			}

			switch msg := nextTaskMsg().(type) {
			case loadTaskFinished:
				_, _ = fmt.Fprintln(pl.w, "")
				break outer
			case LoadTaskStatusSearchingFiles:
				_, _ = fmt.Fprintln(pl.w, "Searching for files...")
			case LoadTaskStatusParsingFiles:
				_, _ = fmt.Fprintln(pl.w, "Parsing files...")
			default:
				if newModel, ok := resultModel.TaskUpdate(msg).(loadTasksModel); ok {
					resultModel = newModel
				}
			}
		}
	}

	if resultModel.err != nil {
		return nil, resultModel.err
	}

	return &resultModel, nil
}

func (m loadTasksModel) Init() tea.Cmd {
	return tea.Batch(
		func() tea.Msg {
			return m.spinner.Tick()
		},
		m.nextTaskMsg,
	)
}

func (m loadTasksModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.err != nil {
		return m, tea.Quit
	}

	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case loadTaskFinished:
		m.clear = true
		return m, tea.Quit
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "crtl+d":
			m.err = errors.New("aborted")
			return m, tea.Quit
		}
	}

	if m, ok := m.TaskUpdate(msg).(loadTasksModel); ok {
		return m, m.nextTaskMsg
	}

	return m, nil
}

func (m loadTasksModel) TaskUpdate(msg tea.Msg) tea.Model {
	switch msg := msg.(type) {

	case LoadTaskError:
		m.err = msg.Err

	// status
	case LoadTaskStatusSearchingFiles:
		m.filename = ""
		m.status = "Searching for files..."
	case LoadTaskStatusParsingFiles:
		m.filename = ""
		m.status = "Parsing files..."

	// filename
	case LoadTaskSearchingFolder:
		m.filename = msg.Folder
	case LoadTaskParsingFile:
		m.filename = msg.Filename

	// results
	case LoadTaskFoundFile:
		m.files = append(m.files, msg.Filename)
	case LoadTaskFoundTask:
		m.tasks = append(m.tasks, msg.Task)

	default:
		return nil
	}

	return m
}

func (m loadTasksModel) View() (s string) {
	if m.clear {
		return
	}

	s += m.spinner.View()
	s += " "

	s += m.status

	if m.filename != "" {
		s += fmt.Sprintf(" (%s)", m.filename)
	}

	return
}

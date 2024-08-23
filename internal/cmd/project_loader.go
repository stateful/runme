package cmd

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/stateful/runme/v3/pkg/project"
)

type projectLoader struct {
	allowUnknown bool
	allowUnnamed bool
	cwd          string
	ctx          context.Context
	w            io.Writer
	r            io.Reader
	isTerminal   bool
}

func newProjectLoader(cmd *cobra.Command, allowUnknown, allowUnnamed bool) (*projectLoader, error) {
	ofd := os.Stdout.Fd()

	if int(ofd) < 0 {
		return nil, fmt.Errorf("invalid file descriptor due to restricted environments, redirected standard output, system configuration issues, or testing/simulation setups")
	}

	return &projectLoader{
		allowUnknown: allowUnknown,
		allowUnnamed: allowUnnamed,
		cwd:          getCwd(),
		ctx:          cmd.Context(),
		w:            cmd.OutOrStdout(),
		r:            cmd.InOrStdin(),
		isTerminal:   isTerminal(ofd) && isTerminal(os.Stdin.Fd()),
	}, nil
}

func (pl projectLoader) LoadFiles(proj *project.Project) ([]string, error) {
	files, _, err := pl.load(proj, true)
	return files, err
}

func (pl projectLoader) loadSorted(proj *project.Project) ([]project.Task, error) {
	_, tasks, err := pl.load(proj, false)

	project.SortByProximity(tasks, pl.cwd)

	return tasks, err
}

func (pl projectLoader) LoadTasks(proj *project.Project) ([]project.Task, error) {
	tasks, err := pl.loadSorted(proj)
	if err != nil {
		return nil, err
	}

	if pl.allowUnknown && pl.allowUnnamed {
		return tasks, nil
	}

	filtered := make([]project.Task, 0, len(tasks))

	for _, task := range tasks {
		if !pl.allowUnknown && task.CodeBlock.IsUnknown() {
			continue
		}

		if !pl.allowUnnamed && task.CodeBlock.IsUnnamed() {
			continue
		}

		filtered = append(filtered, task)
	}

	return filtered, nil
}

func (pl projectLoader) LoadAllTasks(proj *project.Project) ([]project.Task, error) {
	tasks, err := pl.loadSorted(proj)
	return tasks, err
}

func (pl projectLoader) load(proj *project.Project, onlyFiles bool) ([]string, []project.Task, error) {
	if pl.isTerminal {
		return pl.loadInTerminal(proj, onlyFiles)
	}
	return pl.loadWithoutTerminal(proj, onlyFiles)
}

func (pl projectLoader) loadInTerminal(proj *project.Project, onlyFiles bool) ([]string, []project.Task, error) {
	eventc := make(chan project.LoadEvent)

	go proj.LoadWithOptions(pl.ctx, eventc, project.LoadOptions{OnlyFiles: onlyFiles})

	nextTaskMsg := func() tea.Msg {
		event, ok := <-eventc
		if !ok {
			return loadTasksFinished{}
		}
		return loadTasksEvent{Event: event}
	}

	m := newLoadTasksModel(nextTaskMsg)

	p := tea.NewProgram(m, tea.WithOutput(pl.w), tea.WithInput(pl.r))
	result, err := p.Run()
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}

	resultModel := result.(loadTasksModel)

	if resultModel.err != nil {
		return nil, nil, resultModel.err
	}

	return resultModel.files, resultModel.tasks, nil
}

func (pl projectLoader) loadWithoutTerminal(proj *project.Project, onlyFiles bool) ([]string, []project.Task, error) {
	eventc := make(chan project.LoadEvent)

	go proj.Load(pl.ctx, eventc, onlyFiles)

	var (
		files []string
		tasks []project.Task
	)

	for event := range eventc {
		switch event.Type {
		case project.LoadEventError:
			err := project.ExtractDataFromLoadEvent[project.LoadEventErrorData](event).Err
			return nil, nil, err

		case project.LoadEventFoundFile:
			path := project.ExtractDataFromLoadEvent[project.LoadEventFoundFileData](event).Path
			files = append(files, path)

		case project.LoadEventFoundTask:
			if onlyFiles {
				continue
			}

			task := project.ExtractDataFromLoadEvent[project.LoadEventFoundTaskData](event).Task
			tasks = append(tasks, task)
		}
	}

	return files, tasks, nil
}

type loadTasksEvent struct {
	Event project.LoadEvent
}

type loadTasksFinished struct{}

type loadTasksModel struct {
	spinner spinner.Model

	status   string
	filename string

	finished bool
	err      error

	tasks []project.Task
	files []string

	nextTaskMsg tea.Cmd
}

func newLoadTasksModel(nextTaskMsg tea.Cmd) loadTasksModel {
	return loadTasksModel{
		spinner:     spinner.New(spinner.WithSpinner(spinner.Pulse)),
		nextTaskMsg: nextTaskMsg,
		status:      "Initializing...",
	}
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
	case loadTasksFinished:
		m.finished = true
		return m, tea.Quit

	case loadTasksEvent:
		return m.handleTask(msg.Event)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyCtrlD:
			m.err = errors.New("aborted")
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m loadTasksModel) handleTask(event project.LoadEvent) (tea.Model, tea.Cmd) {
	cmd := m.nextTaskMsg

	switch event.Type {
	case project.LoadEventError:
		data := project.ExtractDataFromLoadEvent[project.LoadEventErrorData](event)
		m.err = data.Err
		cmd = tea.Quit

	case project.LoadEventStartedWalk:
		m.filename = ""
		m.status = "Searching for files..."

	case project.LoadEventFinishedWalk:
		m.filename = ""
		m.status = "Parsing files..."

	case project.LoadEventFoundDir:
		data := project.ExtractDataFromLoadEvent[project.LoadEventFoundDirData](event)
		m.filename = data.Path

	case project.LoadEventStartedParsingDocument:
		data := project.ExtractDataFromLoadEvent[project.LoadEventStartedParsingDocumentData](event)
		m.filename = data.Path

	case project.LoadEventFoundFile:
		data := project.ExtractDataFromLoadEvent[project.LoadEventFoundFileData](event)
		m.files = append(m.files, data.Path)

	case project.LoadEventFoundTask:
		data := project.ExtractDataFromLoadEvent[project.LoadEventFoundTaskData](event)
		m.tasks = append(m.tasks, data.Task)
	}

	return m, cmd
}

func (m loadTasksModel) View() (s string) {
	if m.finished {
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

package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/stateful/runme/internal/client/graphql"
	"github.com/stateful/runme/internal/client/graphql/query"
	"github.com/stateful/runme/internal/log"
	"github.com/stateful/runme/internal/project"
	"go.uber.org/zap"
)

var (
	appStyle  = lipgloss.NewStyle().Margin(1, 2)
	helpStyle = lipgloss.NewStyle().
			PaddingLeft(2).
			Foreground(lipgloss.Color("241"))
	inputStyle = lipgloss.NewStyle().
			MarginLeft(1).
			MarginBottom(2).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("63")).
			Foreground(lipgloss.Color("#87448b"))
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#5d5dd2")).
			MarginLeft(2).
			PaddingLeft(1).
			PaddingRight(1).
			MarginBottom(2)
)

type ListModel struct {
	ctx    context.Context
	client *graphql.Client
	log    *zap.Logger

	repoUser    string
	description string
	suggestions []string
	list        list.Model
	input       textinput.Model
	keys        keyMap
	help        help.Model

	confirmed bool
	loading   bool

	spinner   spinner.Model
	selected  item
	usePrefix bool
	editing   bool

	err error
}

var listKeys = []key.Binding{
	key.NewBinding(
		key.WithKeys("t"),
		key.WithHelp("t", "toggle suffix"),
	),
	key.NewBinding(
		key.WithKeys("e"),
		key.WithHelp("e", "edit suggestion"),
	),
}

func NewListModel(ctx context.Context, description string, repoUser string, client *graphql.Client) ListModel {
	s := spinner.New()
	s.Spinner = spinner.Line

	emptyItems := make([]list.Item, 0)
	delegate := list.NewDefaultDelegate()
	l := list.New(emptyItems, delegate, 0, 0)
	l.SetFilteringEnabled(false)
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return listKeys
	}
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return listKeys
	}

	ti := textinput.New()
	ti.Prompt = ""
	ti.CharLimit = 250
	ti.Width = 50

	return ListModel{
		ctx:         ctx,
		client:      client,
		log:         log.Get().Named("SuggestionsListModel"),
		repoUser:    repoUser,
		description: description,
		list:        l,
		input:       ti,
		keys:        inputKeys,
		help:        help.New(),
		spinner:     s,
		loading:     true,
		usePrefix:   true,
	}
}

func (m ListModel) startSearch() tea.Msg {
	userBranches, err := project.GetUsersBranches(m.repoUser)
	if err != nil {
		// This isn't fatal, we will just defer to the repo branches.
		log.Get().Debug("Error preparing users branches", zap.Any("err", err))
	}
	if len(userBranches) > 100 {
		userBranches = userBranches[:100]
	}

	repoBranches, err := project.GetRepoBranches()
	if err != nil {
		// This isn't fatal, worst case we fall back to generic recommendations.
		log.Get().Debug("Error preparing repository branches", zap.Any("err", err))
	}
	if len(repoBranches) > 100 {
		repoBranches = repoBranches[:100]
	}

	input, err := newGetSuggestedBranch(m.description, userBranches, repoBranches)
	if err != nil {
		return errorMsg{err}
	}

	log.Get().Debug("prepared GetSuggestedBranch", zap.Any("input", input))

	suggestions, err := m.client.GetSuggestedBranch(m.ctx, input)
	if err != nil {
		log.Get().Debug("Server error when getting suggested branch", zap.Any("input", err))
		return errorMsg{err}
	}

	// Because we are passing this directly into the terminal, let's make sure
	// to sanitize it.
	suggestions = sanitizeBranchName(suggestions)

	log.Get().Debug("Success, updating with suggestions")
	return suggestionsMsg{suggestions}
}

func (m ListModel) Init() tea.Cmd {
	return tea.Batch(m.startSearch, m.spinner.Tick)
}

type suggestionsMsg struct {
	suggestions []string
}

type confirmMsg struct {
	response string
}

type errorMsg struct {
	Err error
}

type keyMap struct {
	Save  key.Binding
	Enter key.Binding
	Quit  key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Save, k.Enter, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Save, k.Enter, k.Quit}, // second column
	}
}

var inputKeys = keyMap{
	Save: key.NewBinding(
		key.WithKeys("s", "enter"),
		key.WithHelp("enter/ctrl+s", "save suggest"),
	),
	Quit: key.NewBinding(
		key.WithKeys("esc", "esc", "ctrl+c"),
		key.WithHelp("esc/ctrl+c", "quit"),
	),
}

func (m ListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h, v := appStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
		m.help.Width = msg.Width - h

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		if m.input.Focused() {
			switch msg.Type {
			case tea.KeyEnter, tea.KeyCtrlS:
				m.list.SetItem(m.list.Cursor(), createListItem(removeSpecialChars(m.input.Value()), m.usePrefix))
				m.input.Blur()
				return m, nil
			case tea.KeyCtrlC, tea.KeyEsc:
				m.input.Blur()
				return m, nil
			}

			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
		switch msg.String() {

		case "up":
			m.list.CursorUp()

			return m, nil
		case "down":
			m.list.CursorDown()

			return m, nil
		case "enter":

			if m.loading {
				return m, nil
			}

			selected, ok := m.list.SelectedItem().(item)

			return m, func() tea.Msg {
				if !ok {
					return errorMsg{errors.New("unable to select an item")}
				}

				m.selected = selected

				cwd, err := os.Getwd()
				if err != nil {
					return errorMsg{err}
				}

				cmdSlice := []string{"git", "checkout", "-b", m.selected.title}
				fmt.Printf("Output: %s", cmdSlice)

				cmd := exec.Command(cmdSlice[0], cmdSlice[1:]...)
				cmd.Dir = cwd

				out, err := cmd.CombinedOutput()
				if err != nil {
					return errorMsg{err}
				}

				return confirmMsg{string(out)}
			}
		case "t":
			m := m.TogglePrefixes()
			return m, nil
		case "e":
			selected := m.list.SelectedItem().(item)
			m.input.SetValue(selected.title)
			m.input.Focus()
			return m, nil
		}

		// Cool, what was the actual key pressed?

	case suggestionsMsg:
		m.suggestions = msg.suggestions
		for i, s := range m.suggestions {
			m.list.InsertItem(i, createListItem(s, true))
		}
		m.loading = false
		return m, nil

	case confirmMsg:
		m.confirmed = true
		fmt.Printf("Output: %s", msg.response)
		return m, tea.Quit // see https://github.com/charmbracelet/bubbletea/discussions/273

	case errorMsg:
		m.loading = false
		m.err = msg.Err
		return m, nil // see https://github.com/charmbracelet/bubbletea/discussions/273 // tea.Quit
	}

	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m ListModel) View() string {
	var s string
	if m.err != nil {
		s := fmt.Sprintf("%s\n", ColorError.Render("Error: "+m.err.Error()))
		return s
	}

	if m.confirmed {
		s := fmt.Sprintf("Confirmed: %s", m.selected.title)
		return s
	}

	if m.loading {
		s := fmt.Sprintf("\n%s loading suggestions...\n", m.spinner.View())
		return s
	}

	m.list.Title = "How about one of these:"

	if m.input.Focused() {
		s += titleStyle.Render("Edit suggestion:") + "\n"
		s += inputStyle.Render(m.input.View()) + "\n"
		s += helpStyle.Render(m.help.View(m.keys))
		return appStyle.Render(s)
	}
	s += appStyle.Render(m.list.View())

	return s
}

type item struct {
	suggestion string
	title      string
}

func (i item) Title() string { return i.title }
func (i item) Description() string {
	return "git checkout -b '" + i.title
}
func (i item) FilterValue() string { return i.suggestion }

func (m ListModel) TogglePrefixes() ListModel {
	m.usePrefix = !m.usePrefix
	for i, item := range m.list.Items() {
		m.list.SetItem(i, createListItem(item.FilterValue(), m.usePrefix))
	}
	return m
}

func createListItem(suggestion string, showPrefix bool) item {

	item := item{suggestion: suggestion, title: suggestion}
	if !showPrefix {
		parts := strings.Split(suggestion, "/")
		if len(parts) > 0 {
			item.title = strings.Replace(suggestion, parts[0]+"/", "", 1)
		}
		item.title = strings.Replace(suggestion, parts[0]+"/", "", 1)
	}

	return item
}

func newGetSuggestedBranch(description string, userBranches []project.Branch, repoBranches []project.Branch) (input query.SuggestedBranchInput, _ error) {
	ub := []query.BranchSuggestionInput{}
	for _, x := range userBranches {
		ub = append(ub, query.BranchSuggestionInput{Branch: x.Name, Description: x.Description})
	}

	rb := []query.BranchSuggestionInput{}
	for _, x := range repoBranches {
		rb = append(rb, query.BranchSuggestionInput{Branch: x.Name, Description: x.Description})
	}

	return query.SuggestedBranchInput{
		Description:  description,
		UserBranches: ub,
		RepoBranches: rb,
	}, nil
}
func removeSpecialChars(unsanitized string) string {
	pattern := regexp.MustCompile(`[^A-Za-z\-_\/]+`)
	sanitized := pattern.ReplaceAllString(unsanitized, "")
	return sanitized
}

func sanitizeBranchName(unsanitized []string) []string {
	// Better safe than sorry - the methodology here is simple - if the string
	// contains a character that isn't in our whitelist (alpha, plus dashes)
	// then we omit the suggestion.

	// Edge case is of course that we are left with an empty list.
	isInWhitelist := regexp.MustCompile(`^[A-Za-z-_\/]+$`).MatchString

	sanitized := make([]string, 0)
	for _, s := range unsanitized {
		if isInWhitelist(s) {
			sanitized = append(sanitized, s)
		}
	}
	return sanitized
}

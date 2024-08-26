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
	"github.com/stateful/runme/v3/internal/client/graphql"
	"github.com/stateful/runme/v3/internal/client/graphql/query"
	"github.com/stateful/runme/v3/internal/gitrepo"
	"github.com/stateful/runme/v3/internal/log"
	"go.uber.org/zap"
)

var (
	appStyle      = lipgloss.NewStyle().Margin(1, 2)
	listItemStyle = lipgloss.NewStyle().PaddingLeft(0).Bold(true)
	editHelpStyle = lipgloss.NewStyle().
			PaddingLeft(2).
			Foreground(lipgloss.Color("241"))
	editInputStyle = lipgloss.NewStyle().
			MarginLeft(1).
			MarginBottom(2).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("63")).
			Foreground(lipgloss.Color("#87448b"))
	editTitleStyle = lipgloss.NewStyle().
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

	editInput textinput.Model
	editHelp  help.Model

	confirmed bool
	loading   bool

	spinner   spinner.Model
	selected  item
	usePrefix bool

	err error
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

	ti := textinput.New()
	ti.Prompt = ""
	ti.CharLimit = 250
	ti.Width = 50

	m := ListModel{
		ctx:         ctx,
		client:      client,
		log:         log.Get().Named("SuggestionsListModel"),
		repoUser:    repoUser,
		description: description,
		list:        l,
		editInput:   ti,
		editHelp:    help.New(),
		spinner:     s,
		confirmed:   false,
		loading:     true,
		usePrefix:   true,
	}

	m.list.AdditionalShortHelpKeys = func() []key.Binding {
		return m.KeyMap().ShortHelp()
	}
	m.list.AdditionalFullHelpKeys = func() []key.Binding {
		return m.KeyMap().ShortHelp()
	}

	return m
}

func (m ListModel) Confirmed() bool {
	return m.confirmed
}

func (m ListModel) startSearch() tea.Msg {
	userBranches, err := gitrepo.GetUsersBranches(m.repoUser)
	if err != nil {
		// This isn't fatal, we will just defer to the repo branches.
		log.Get().Debug("Error preparing users branches", zap.Any("err", err))
	}
	if len(userBranches) > 100 {
		userBranches = userBranches[:100]
	}

	repoBranches, err := gitrepo.GetRepoBranches()
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

func (m ListModel) KeyMap() *KeyMap {
	kmap := NewKeyMap()

	kmap.Set("enter", key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	))
	kmap.Set("up", m.list.KeyMap.CursorUp)
	kmap.Set("down", m.list.KeyMap.CursorDown)
	kmap.Set("t", key.NewBinding(
		key.WithKeys("t"),
		key.WithHelp("t", "toggle prefix"),
	))
	kmap.Set("e", key.NewBinding(
		key.WithKeys("e"),
		key.WithHelp("e", "edit branch name"),
	))

	return kmap
}

func (m ListModel) EditKeyMap() *KeyMap {
	editInputKeys := NewKeyMap()

	editInputKeys.Set("enter", key.NewBinding(
		key.WithKeys("enter", "ctrl+s"),
		key.WithHelp("enter/ctrl+s", "save changes"),
	))
	editInputKeys.Set("esc", key.NewBinding(
		key.WithKeys("esc", "esc", "ctrl+c"),
		key.WithHelp("esc/ctrl+c", "quit"),
	))

	return editInputKeys
}

func (m ListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h, v := appStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
		m.editHelp.Width = msg.Width - h

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		if m.editInput.Focused() {

			editKmap := m.EditKeyMap()
			switch {
			case editKmap.Matches(msg, "enter"):
				selected := m.list.SelectedItem().(item)
				selected.branchName = removeSpecialChars(m.editInput.Value())
				m.list.SetItem(m.list.Cursor(), selected)
				m.editInput.Blur()
				return m, nil
			case editKmap.Matches(msg, "esc"):
				m.editInput.Blur()
				return m, nil
			}

			m.editInput, cmd = m.editInput.Update(msg)
			return m, cmd
		}
		kmap := m.KeyMap()
		switch {

		case kmap.Matches(msg, "up"):
			m.list.CursorUp()

			return m, nil
		case kmap.Matches(msg, "down"):
			m.list.CursorDown()

			return m, nil
		case kmap.Matches(msg, "enter"):

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

				cmdSlice := []string{"git", "checkout", "-b", m.selected.Suggestion()}
				_, _ = fmt.Printf("Output: %s", cmdSlice)

				cmd := exec.Command(cmdSlice[0], cmdSlice[1:]...)
				cmd.Dir = cwd

				out, err := cmd.CombinedOutput()
				if err != nil {
					return errorMsg{err}
				}

				return confirmMsg{string(out)}
			}
		case kmap.Matches(msg, "t"):
			m := m.TogglePrefixes()
			return m, nil
		case kmap.Matches(msg, "e"):
			selected := m.list.SelectedItem().(item)
			m.editInput.SetValue(selected.branchName)
			m.editInput.Focus()
			return m, nil
		}

	case suggestionsMsg:
		m.suggestions = msg.suggestions
		for _, s := range m.suggestions {
			m.list.InsertItem(0, createListItem(s, true))
		}
		m.loading = false
		return m, nil

	case confirmMsg:
		m.confirmed = true
		_, _ = fmt.Printf("Output: %s", msg.response)
		return m, tea.Quit

	case errorMsg:
		m.loading = false
		m.err = msg.Err
		return m, tea.Quit // now fixed https://github.com/charmbracelet/bubbletea/issues/274
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
		s := fmt.Sprintf("Confirmed: %s", m.selected.branchName)
		return s
	}

	if m.loading {
		s := fmt.Sprintf("\n%s loading suggestions...\n", m.spinner.View())
		return s
	}

	if m.editInput.Focused() {
		s += editTitleStyle.Render("Edit branch name:") + "\n"
		s += editInputStyle.Render(m.editInput.View()) + "\n"
		s += editHelpStyle.Render(m.editHelp.View(m.EditKeyMap()))
		return appStyle.Render(s)
	}
	m.list.Title = "How about one of these (generated by AI):"
	s += appStyle.Render(m.list.View())

	return s
}

type item struct {
	branchName string
	usePrefix  bool
}

func (i item) Title() string { return listItemStyle.Render(i.Suggestion()) }
func (i item) Description() string {
	return fmt.Sprintf("git checkout -b '%v'", i.Suggestion())
}
func (i item) FilterValue() string { return i.branchName }
func (i item) Suggestion() string {
	if i.usePrefix {
		return i.branchName
	}

	parts := strings.SplitN(i.branchName, "/", 2)

	if len(parts) == 2 {
		return parts[1]
	}

	return i.branchName
}

func (m ListModel) TogglePrefixes() ListModel {
	m.usePrefix = !m.usePrefix
	for idx, i := range m.list.Items() {
		item := i.(item)
		item.usePrefix = !item.usePrefix
		m.list.SetItem(idx, item)
	}
	return m
}

func createListItem(branchName string, showPrefix bool) item {
	return item{branchName: branchName, usePrefix: showPrefix}
}

func newGetSuggestedBranch(description string, userBranches []gitrepo.Branch, repoBranches []gitrepo.Branch) (input query.SuggestedBranchInput, _ error) {
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
	pattern := regexp.MustCompile(`[^A-Za-z0-9\-_\/]+`)
	sanitized := pattern.ReplaceAllString(unsanitized, "")
	return sanitized
}

func sanitizeBranchName(unsanitized []string) []string {
	// Better safe than sorry - the methodology here is simple - if the string
	// contains a character that isn't in our whitelist (alpha, plus dashes)
	// then we omit the suggestion.

	// Edge case is of course that we are left with an empty list.
	isInWhitelist := regexp.MustCompile(`^[A-Za-z0-9-_\/]+$`).MatchString

	sanitized := make([]string, 0)
	for _, s := range unsanitized {
		if isInWhitelist(s) {
			sanitized = append(sanitized, s)
		}
	}
	return sanitized
}

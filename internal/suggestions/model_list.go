package suggestions

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/stateful/runme/client/graphql"
	"github.com/stateful/runme/client/graphql/query"
	"github.com/stateful/runme/internal/log"
	"github.com/stateful/runme/internal/project"
	"github.com/stateful/runme/internal/renderer"

	// "github.com/stateful/runme/internal/renderer"
	"go.uber.org/zap"
)

var (
	appStyle            = lipgloss.NewStyle().Padding(1, 2)
	listItemSidePadding = 0
	listItemStyle       = lipgloss.NewStyle().PaddingLeft(listItemSidePadding).Bold(true)
	listItemHelpStyle   = lipgloss.NewStyle().Align(lipgloss.Right)
)

type ListModel struct {
	ctx    context.Context
	client *graphql.Client
	log    *zap.Logger

	repoUser    string
	description string
	suggestions []string
	list        list.Model

	confirmed bool
	loading   bool

	spinner  spinner.Model
	selected string

	err error
}

func NewListModel(ctx context.Context, description string, repoUser string, client *graphql.Client) ListModel {
	s := spinner.NewModel()
	s.Spinner = spinner.Line

	emptyItems := make([]list.Item, 0)
	delegate := list.NewDefaultDelegate()
	l := list.NewModel(emptyItems, delegate, 0, 0) // renderer.MaxWidth, renderer.MaxHeight)
	l.SetFilteringEnabled(false)
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)

	return ListModel{
		ctx:         ctx,
		client:      client,
		log:         log.Get().Named("SuggestionsListModel"),
		repoUser:    repoUser,
		description: description,
		list:        l,
		spinner:     s,
		loading:     true,
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

func (m ListModel) KeyMap() *renderer.KeyMap {
	kmap := renderer.NewKeyMap()

	kmap.Set("enter", key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	))
	kmap.Set("up", m.list.KeyMap.CursorUp)
	kmap.Set("down", m.list.KeyMap.CursorDown)

	return kmap
}

func (m ListModel) Init() tea.Cmd {
	return tea.Batch(m.startSearch, spinner.Tick)
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

func (m ListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h, v := appStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		kmap := m.KeyMap()
		if kmap.Matches(msg, "enter") {
			if m.loading {
				return m, nil
			}

			selected, ok := m.list.SelectedItem().(item)

			return m, func() tea.Msg {
				if !ok {
					return errorMsg{errors.New("unable to select an item")}
				}

				m.selected = string(selected.suggestion)

				cwd, err := os.Getwd()
				if err != nil {
					return errorMsg{err}
				}

				cmdSlice := []string{"git", "checkout", "-b", m.selected}
				fmt.Printf("Output: %s", cmdSlice)

				cmd := exec.Command(cmdSlice[0], cmdSlice[1:]...)
				cmd.Dir = cwd

				out, err := cmd.CombinedOutput()
				if err != nil {
					return errorMsg{err}
				}

				return confirmMsg{string(out)}
			}
		}

	case suggestionsMsg:
		m.suggestions = msg.suggestions
		for i, s := range m.suggestions {
			m.list.InsertItem(0, createListItem(s, i))
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

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m ListModel) View() string {
	if m.err != nil {
		s := fmt.Sprintf("%s\n", renderer.ColorError.Render("Error: "+m.err.Error()))
		return s
	}

	if m.confirmed {
		s := fmt.Sprintf("Confirmed: %s", m.selected)
		return s
	}

	if m.loading {
		s := fmt.Sprintf("\n%s loading suggestions...\n", m.spinner.View())
		return s
	}

	m.list.Title = "How about one of these:"
	return appStyle.Render(m.list.View())
}

type item struct {
	suggestion string
}

func (m item) FilterValue() string { return m.suggestion }
func (m item) Title() string       { return listItemStyle.Render(m.suggestion) }
func (m item) Description() string {
	return listItemHelpStyle.Render("git checkout -b '" + m.suggestion + "'")
}

func createListItem(suggestion string, index int) item {
	return item{suggestion: suggestion}
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

package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/elliotchance/orderedmap"
	"github.com/stateful/runme/v3/internal/log"
	"go.uber.org/zap"
)

const (
	MaxWidth = 120
)

func Width(width int) int {
	if width > MaxWidth {
		return MaxWidth
	}
	return width
}

type Option func(*Model)

func WithoutHelp() Option {
	return func(m *Model) {
		m.disableHelp = true
	}
}

type Model struct {
	Child  tea.Model
	KeyMap *KeyMap
	Styles *Styles

	disableHelp bool
	err         error
	help        help.Model
	log         *zap.Logger
}

func NewModel(
	child tea.Model,
	keyMap *KeyMap,
	styles *Styles,
	opts ...Option,
) Model {
	m := Model{
		Child:  child,
		KeyMap: keyMap,
		Styles: styles,
		help:   help.New(),
		log:    log.Get().Named("renderer.Model"),
	}

	for _, opt := range opts {
		opt(&m)
	}

	return m
}

func (m Model) Init() tea.Cmd {
	return m.Child.Init()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.help.Width = Width(msg.Width)

	case tea.KeyMsg:
		m.log.Debug("received KeyMsg", zap.String("key", msg.String()))
		switch {
		case m.KeyMap.Matches(msg, "quit"):
			return m, tea.Quit
		case m.KeyMap.Matches(msg, "less"):
			fallthrough
		case m.KeyMap.Matches(msg, "more"):
			m.help.ShowAll = !m.help.ShowAll
			return m, nil
		}

	case ErrorMsg:
		m.log.Debug("received ErrorMsg", zap.String("error", msg.Err.Error()))
		m.err = msg.Err
		if msg.Exit {
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.Child, cmd = m.Child.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	var b strings.Builder

	if m.err != nil {
		_, _ = b.WriteString(m.Styles.Error.Render("Error: " + m.err.Error()))
	} else {
		_, _ = b.WriteString(m.Styles.Child.Render(m.Child.View()))
	}

	kmap := m.KeyMap.Copy()
	if p, ok := m.Child.(KeyMapProvider); ok {
		kmap.Merge(p.KeyMap())
	}

	if !m.disableHelp {
		_, _ = b.WriteString(m.Styles.Help.Render(m.help.View(kmap)))
	} else {
		_, _ = b.WriteString("\n")
	}

	return b.String()
}

type ErrorMsg struct {
	Err  error
	Exit bool
}

type Styles struct {
	Child   lipgloss.Style
	Error   lipgloss.Style
	Help    lipgloss.Style
	Success lipgloss.Style
}

var (
	ColorError   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#FF0000", Dark: "#FF0000"})
	ColorSuccess = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "32", Dark: "42"})
	ColorHelp    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#B2B2B2", Dark: "#4A4A4A"})
)

var DefaultStyles = &Styles{
	Child:   lipgloss.NewStyle().Padding(1, 0, 0, 0),
	Error:   lipgloss.NewStyle().Inherit(ColorError).Padding(1, 0),
	Help:    lipgloss.NewStyle().Padding(1, 0, 1, 2),
	Success: lipgloss.NewStyle().Inherit(ColorSuccess).Padding(1, 0),
}

var MinimalKeyMap = func() *KeyMap {
	m := orderedmap.NewOrderedMap()
	m.Set("quit", key.NewBinding(
		key.WithKeys("ctrl+c"),
		key.WithHelp("ctrl+c", "quit"),
	))
	return &KeyMap{OrderedMap: m}
}()

type KeyMapProvider interface {
	KeyMap() *KeyMap
}

type KeyMap struct {
	*orderedmap.OrderedMap
}

func NewKeyMap() *KeyMap {
	return &KeyMap{OrderedMap: orderedmap.NewOrderedMap()}
}

func (m *KeyMap) Add(name string, binding key.Binding) {
	m.Set(name, binding)
}

func (m KeyMap) Copy() *KeyMap {
	return &KeyMap{
		OrderedMap: m.OrderedMap.Copy(),
	}
}

func (m *KeyMap) Merge(kmap *KeyMap) {
	for pair := kmap.Front(); pair != nil; pair = pair.Next() {
		m.Set(pair.Key, pair.Value)
	}
}

func (m KeyMap) Matches(msg tea.KeyMsg, keyStr string) bool {
	v, ok := m.Get(keyStr)
	if !ok {
		return false
	}
	return key.Matches(msg, v.(key.Binding))
}

var _ help.KeyMap = (*KeyMap)(nil)

func (m KeyMap) ShortHelp() []key.Binding {
	result := make([]key.Binding, 0, m.Len())
	for pair := m.Front(); pair != nil; pair = pair.Next() {
		if v, _ := pair.Key.(string); v == "less" {
			continue
		}
		result = append(result, pair.Value.(key.Binding))
	}
	return result
}

func (m KeyMap) FullHelp() [][]key.Binding {
	result := [2][]key.Binding{}
	idx := 0
	for pair := m.Front(); pair != nil; pair = pair.Next() {
		if v, _ := pair.Key.(string); v == "more" {
			continue
		}
		result[idx%2] = append(result[idx%2], pair.Value.(key.Binding))
		idx++
	}
	return result[:]
}

func Cmd(msg tea.Msg) tea.Cmd {
	return func() tea.Msg { return msg }
}

package prompt

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stateful/runme/internal/log"
	"go.uber.org/zap"
)

type InputModel struct {
	Text  string
	done  bool
	input textinput.Model
	log   *zap.Logger
}

func NewInputModel(text string) InputModel {
	input := textinput.NewModel()
	input.Prompt = ""

	return InputModel{
		Text:  text,
		input: input,
		log:   log.Get().Named("prompt.InputModel"),
	}
}

func (m InputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m InputModel) Focus() tea.Cmd {
	return func() tea.Msg { return focusMsg{} }
}

func (m InputModel) Update(msg tea.Msg) (InputModel, tea.Cmd) {
	var (
		cmds []tea.Cmd
		cmd  tea.Cmd
	)

	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	switch msg := msg.(type) { // revive:disable-line
	case tea.KeyMsg:
		if m.done || !m.input.Focused() {
			break
		}

		if msg.Type == tea.KeyEnter {
			cmds = append(cmds, func() tea.Msg { return Done{Value: m.input.Value()} })
			m.done = true
			m.input.Blur()
		}

	case focusMsg:
		cmds = append(cmds, m.input.Focus())
	}

	return m, tea.Batch(cmds...)
}

func (m InputModel) View() string {
	var b strings.Builder
	if _, err := b.WriteString(m.Text + " " + m.input.View()); err != nil {
		m.log.Error(err.Error())
	}
	return b.String()
}

type Done struct {
	Value string
}

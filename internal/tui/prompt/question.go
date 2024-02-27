package prompt

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stateful/runme/v3/internal/log"
	"go.uber.org/zap"
)

type QuestionModel struct {
	Text  string
	done  bool
	input textinput.Model
	log   *zap.Logger
}

func NewQuestionModel(text string) QuestionModel {
	input := textinput.New()
	input.CharLimit = 1
	input.Placeholder = "Y"
	input.Prompt = ""
	input.Width = 1

	return QuestionModel{
		Text:  text,
		input: input,
		log:   log.Get().Named("prompt.QuestionModel"),
	}
}

func (m QuestionModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m QuestionModel) Focus() tea.Cmd {
	return func() tea.Msg { return focusMsg{} }
}

func (m QuestionModel) Update(msg tea.Msg) (QuestionModel, tea.Cmd) {
	var (
		cmds []tea.Cmd
		cmd  tea.Cmd
	)

	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	switch msg.(type) { // revive:disable-line
	case tea.KeyMsg:
		if m.done || !m.input.Focused() {
			break
		}

		m.done = true
		m.input.Blur()

		m.log.Debug("received KeyMsg", zap.String("value", m.input.Value()))

		val := strings.ToLower(m.input.Value())
		confirmed := val == "y" || val == ""
		cmds = append(cmds, func() tea.Msg { return Confirmed{Value: confirmed} })

	case focusMsg:
		cmds = append(cmds, m.input.Focus())
	}

	return m, tea.Batch(cmds...)
}

func (m QuestionModel) View() string {
	var b strings.Builder
	_, _ = b.WriteString(m.Text + " [Y/n] " + m.input.View())
	return b.String()
}

type Confirmed struct {
	Value bool
}

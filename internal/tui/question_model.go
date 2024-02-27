package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stateful/runme/v3/internal/log"
	"github.com/stateful/runme/v3/internal/tui/prompt"
	"go.uber.org/zap"
)

type StandaloneQuestionModel struct {
	tea.Model
	confirmed bool
	log       *zap.Logger
}

func NewStandaloneQuestionModel(
	text string,
	keyMap *KeyMap,
	styles *Styles,
	opts ...Option,
) StandaloneQuestionModel {
	return StandaloneQuestionModel{
		Model: NewModel(
			questionWrapModel{prompt.NewQuestionModel(text)},
			keyMap,
			styles,
			append([]Option{WithoutHelp()}, opts...)...,
		),
		log: log.Get().Named("renderer.StandaloneQuestionModel"),
	}
}

func (m StandaloneQuestionModel) Confirmed() bool { return m.confirmed }

func (m StandaloneQuestionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) { // revive:disable-line
	case prompt.Confirmed:
		m.log.Debug("confirmed message received", zap.Bool("value", msg.Value))
		m.confirmed = msg.Value
		return m, tea.Quit
	}

	var cmd tea.Cmd
	m.Model, cmd = m.Model.Update(msg)
	return m, cmd
}

// questionWrapModel is a wrapper for prompt.Model
// to make it adherent to tea.Model interface.
type questionWrapModel struct {
	prompt.QuestionModel
}

func (m questionWrapModel) Init() tea.Cmd {
	var cmds []tea.Cmd
	cmds = append(cmds, m.QuestionModel.Init(), m.QuestionModel.Focus())
	return tea.Batch(cmds...)
}

func (m questionWrapModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.QuestionModel, cmd = m.QuestionModel.Update(msg)
	return m, cmd
}

func (m questionWrapModel) View() string {
	return m.QuestionModel.View() + "\n"
}

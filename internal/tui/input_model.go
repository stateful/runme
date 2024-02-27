package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stateful/runme/v3/internal/log"
	"github.com/stateful/runme/v3/internal/tui/prompt"
	"go.uber.org/zap"
)

type StandaloneInputModel struct {
	tea.Model
	value    string
	pristine bool
	log      *zap.Logger
}

func NewStandaloneInputModel(
	inputParams prompt.InputParams,
	keyMap *KeyMap,
	styles *Styles,
	opts ...Option,
) StandaloneInputModel {
	return StandaloneInputModel{
		Model: NewModel(
			inputWrapModel{prompt.NewInputModel(inputParams)},
			keyMap,
			styles,
			append([]Option{WithoutHelp()}, opts...)...,
		),
		pristine: true,
		log:      log.Get().Named("renderer.StandaloneInputModel"),
	}
}

func (m StandaloneInputModel) Value() (string, bool) { return m.value, !m.pristine }

func (m StandaloneInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) { // revive:disable-line
	case prompt.Done:
		m.log.Debug("finished writing content", zap.Int("len", len(msg.Value)))
		m.pristine = !m.pristine
		m.value = msg.Value
		return m, tea.Quit
	}

	var cmd tea.Cmd
	m.Model, cmd = m.Model.Update(msg)
	return m, cmd
}

// inputWrapModel is a wrapper for prompt.InputModel
// to make it adherent to tea.Model interface.
type inputWrapModel struct {
	prompt.InputModel
}

func (m inputWrapModel) Init() tea.Cmd {
	var cmds []tea.Cmd
	cmds = append(cmds, m.InputModel.Init(), m.InputModel.Focus())
	return tea.Batch(cmds...)
}

func (m inputWrapModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.InputModel, cmd = m.InputModel.Update(msg)
	return m, cmd
}

func (m inputWrapModel) View() string {
	return m.InputModel.View() + "\n"
}

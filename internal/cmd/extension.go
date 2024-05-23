package cmd

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/stateful/runme/v3/internal/extension"
	"github.com/stateful/runme/v3/internal/log"
	"github.com/stateful/runme/v3/internal/tui"
	"github.com/stateful/runme/v3/internal/tui/prompt"

	"go.uber.org/zap"
)

func extensionCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "extension",
		Short: "Check your Runme VS Code extension status",
		RunE: func(cmd *cobra.Command, args []string) error {
			model := tui.NewModel(
				newExtensionerModel(force),
				tui.MinimalKeyMap,
				tui.DefaultStyles,
			)
			_, err := newProgram(cmd, model).Start()
			return err
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "updates extension if it's already installed")

	return cmd
}

type extCheckMsg struct {
	Installed bool
	Name      string
	Err       error
}

type prepExtInstallationMsg struct{}

type extUpdateMsg struct {
	Updated bool
	Name    string
	Err     error
}

type extensionerModel struct {
	force       bool
	extensioner extension.Extensioner
	loading     bool
	loadingMsg  string
	spinner     spinner.Model
	successMsg  string
	prompting   bool
	prompt      prompt.QuestionModel
	log         *zap.Logger
}

func newExtensionerModel(force bool) extensionerModel {
	s := spinner.New()
	s.Spinner = spinner.Line

	return extensionerModel{
		force:       force,
		extensioner: extension.New(fStateful),
		prompt:      prompt.NewQuestionModel("Do you want to install the extension?"),
		spinner:     s,
		loading:     true,
		loadingMsg:  "checking status of the extension...",
		log:         log.Get().Named("command.extensionerModel"),
	}
}

func (m extensionerModel) Init() tea.Cmd {
	return tea.Batch(
		m.prompt.Init(),
		func() tea.Msg {
			fullName, installed, err := m.extensioner.IsInstalled()
			return extCheckMsg{
				Installed: installed,
				Name:      fullName,
				Err:       err,
			}
		},
	)
}

func (m extensionerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case prompt.Confirmed:
		m.prompting = false
		if msg.Value {
			return m, func() tea.Msg { return prepExtInstallationMsg{} }
		}
		m.successMsg = fmt.Sprintf("You can install the extension manually using: %q", m.extensioner.InstallCommand())
		return m, tea.Quit

	case prepExtInstallationMsg:
		m.loading = true
		m.loadingMsg = "installing the extension..."
		return m, m.installExtension

	case extCheckMsg:
		m.loading = false

		if msg.Err != nil {
			return m, tui.Cmd(tui.ErrorMsg{Err: msg.Err, Exit: true})
		}

		if msg.Installed && !m.force {
			m.successMsg = fmt.Sprintf(`It looks like you're set with %s 🙌 `, msg.Name)
			return m, tea.Quit
		}

		if msg.Installed && m.force {
			m.loading = true
			m.loadingMsg = "updating the extension..."
			return m, func() tea.Msg {
				if err := m.extensioner.Update(); err != nil {
					return extUpdateMsg{Err: err}
				}

				updatedFullName, _, err := m.extensioner.IsInstalled()
				if err != nil {
					return extUpdateMsg{
						Err: err,
					}
				}
				return extUpdateMsg{
					Updated: true,
					Name:    updatedFullName,
				}
			}
		}

		if !msg.Installed && !m.force {
			m.prompting = true
			return m, m.prompt.Focus()
		}

		if !msg.Installed && m.force {
			return m, func() tea.Msg { return prepExtInstallationMsg{} }
		}

		return m, nil

	case extUpdateMsg:
		m.log.Debug("received extUpdateMsg", zap.Any("msg", msg))

		m.loading = false
		m.prompting = false

		if msg.Err != nil {
			return m, tui.Cmd(tui.ErrorMsg{Err: msg.Err, Exit: true})
		}

		if msg.Updated {
			m.successMsg = fmt.Sprintf(`Successfully updated to %s 🙌 `, msg.Name)
		}

		return m, tea.Quit
	}

	var cmd tea.Cmd
	m.prompt, cmd = m.prompt.Update(msg)
	return m, cmd
}

func (m extensionerModel) installExtension() tea.Msg {
	if err := m.extensioner.Install(); err != nil {
		return extUpdateMsg{Err: err}
	}
	installedFullName, _, err := m.extensioner.IsInstalled()
	if err != nil {
		return extUpdateMsg{Err: err}
	}
	return extUpdateMsg{
		Updated: true,
		Name:    installedFullName,
	}
}

func (m extensionerModel) View() string {
	var s string

	if m.loading {
		s += m.spinner.View() + " "
		s += m.loadingMsg + "\n"
	} else if m.prompting {
		s += m.prompt.View() + "\n"
	} else {
		s += tui.ColorSuccess.Render(m.successMsg) + "\n"
	}

	return s
}

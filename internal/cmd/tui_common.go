package cmd

import (
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

type program struct {
	*tea.Program
	out io.Writer
}

func (p *program) Start() (tea.Model, error) {
	if f, ok := p.out.(*os.File); ok && !isTerminal(f.Fd()) {
		go p.Quit()
	}
	m, err := p.Program.Run()
	return m, err
}

func newProgramWithOutputs(output io.Writer, input io.Reader, model tea.Model, opts ...tea.ProgramOption) *program {
	if output != nil {
		opts = append(opts, tea.WithOutput(output))
	}

	if input != nil && isTerminal(os.Stdin.Fd()) {
		opts = append(opts, tea.WithInput(input))
	}

	return &program{
		Program: tea.NewProgram(
			model,
			opts...,
		),
		out: output,
	}
}

func newProgram(cmd *cobra.Command, model tea.Model) *program {
	return newProgramWithOutputs(cmd.OutOrStdout(), cmd.InOrStdin(), model)
}

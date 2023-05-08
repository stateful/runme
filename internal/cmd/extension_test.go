package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/golang/mock/gomock"
	"github.com/stateful/runme/internal/extension"
	"github.com/stateful/runme/internal/tui"
	"github.com/stretchr/testify/require"
)

func Test_extensionerModel(t *testing.T) {
	testCases := []struct {
		Name   string
		Force  bool
		Prep   func(*extension.MockExtensioner)
		Expect func(*testing.T, fmt.Stringer)
	}{
		{
			Name: "no code command",
			Prep: func(m *extension.MockExtensioner) {
				m.EXPECT().IsInstalled().Return("", false, errors.New(`command "code" not found`))
			},
			Expect: func(t *testing.T, s fmt.Stringer) {
				require.Contains(t, s.String(), `command "code" not found`)
			},
		},
		{
			Name: "extension already installed",
			Prep: func(m *extension.MockExtensioner) {
				m.EXPECT().IsInstalled().Return("stateful.ext@1.0.0", true, nil)
			},
			Expect: func(t *testing.T, out fmt.Stringer) {
				require.Contains(t, out.String(), "It looks like you're set with stateful.ext@1.0.0")
			},
		},
		{
			Name:  "install extension with force",
			Force: true,
			Prep: func(m *extension.MockExtensioner) {
				m.EXPECT().IsInstalled().Return("", false, nil)
				m.EXPECT().Install().Return(nil)
				m.EXPECT().IsInstalled().Return("stateful.ext@1.0.0", true, nil)
			},
			Expect: func(t *testing.T, out fmt.Stringer) {
				require.Contains(t, out.String(), "Successfully updated to stateful.ext@1.0.0")
			},
		},
		{
			Name:  "update extension",
			Force: true,
			Prep: func(m *extension.MockExtensioner) {
				m.EXPECT().IsInstalled().Return("stateful.ext@1.0.0", true, nil)
				m.EXPECT().Update().Return(nil)
				m.EXPECT().IsInstalled().Return("stateful.ext@2.0.0", true, nil)
			},
			Expect: func(t *testing.T, out fmt.Stringer) {
				require.Contains(t, out.String(), "Successfully updated to stateful.ext@2.0.0")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := extension.NewMockExtensioner(ctrl)
			tc.Prep(m)

			revert := swapExtensioner(m)
			defer revert()

			model := tui.NewModel(
				newExtensionerModel(tc.Force),
				tui.MinimalKeyMap,
				tui.DefaultStyles,
			)

			inR, inW := io.Pipe()
			go func() {
				<-time.After(time.Second)
				inW.Write([]byte{'q'})
			}()
			out := &cleanBuffer{Buffer: bytes.NewBuffer(nil)}
			p := tea.NewProgram(model, tea.WithInput(inR), tea.WithOutput(out))
			_, err := p.Run()
			require.NoError(t, err)
			tc.Expect(t, out)
		})
	}
}

func Test_extensionerModel_prompt_yes(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := extension.NewMockExtensioner(ctrl)
	mock.EXPECT().IsInstalled().Return("", false, nil)
	mock.EXPECT().Install().Return(nil)
	mock.EXPECT().IsInstalled().Return("stateful.ext@1.0.0", true, nil)

	revert := swapExtensioner(mock)
	defer revert()

	model := tui.NewModel(
		newExtensionerModel(false),
		tui.MinimalKeyMap,
		tui.DefaultStyles,
	)

	inR, inW := io.Pipe()
	go func() {
		<-time.After(time.Millisecond * 250)
		_, _ = inW.Write([]byte{'Y'})
		<-time.After(time.Millisecond * 250)
		_, _ = inW.Write([]byte{'Y'})
	}()
	out := &cleanBuffer{Buffer: bytes.NewBuffer(nil)}
	p := tea.NewProgram(model, tea.WithInput(inR), tea.WithOutput(out))
	_, err := p.Run()
	require.NoError(t, err)
	require.Contains(t, out.String(), "Successfully updated to stateful.ext@1.0.0")
}

func Test_extensionerModel_prompt_no(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := extension.NewMockExtensioner(ctrl)
	mock.EXPECT().IsInstalled().Return("", false, nil)

	revert := swapExtensioner(mock)
	defer revert()

	model := tui.NewModel(
		newExtensionerModel(false),
		tui.MinimalKeyMap,
		tui.DefaultStyles,
	)

	inR, inW := io.Pipe()
	go func() {
		<-time.After(time.Millisecond * 250)
		_, _ = inW.Write([]byte{'N'})
	}()
	out := &cleanBuffer{Buffer: bytes.NewBuffer(nil)}
	p := tea.NewProgram(model, tea.WithInput(inR), tea.WithOutput(out))
	_, err := p.Run()
	require.NoError(t, err)
	require.Contains(t, out.String(), "You can install the extension manually using")
}

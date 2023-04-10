package cmd

import (
	"context"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/stateful/runme/internal/tui"
)

func authCmd() *cobra.Command {
	cmd := &cobra.Command{
		Hidden: true,
		Use:    "auth",
		Short:  "Log in and out of your Stateful",
	}

	cmd.AddCommand(loginCmd())
	cmd.AddCommand(logoutCmd())

	return cmd
}

func loginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "login",
		RunE: func(cmd *cobra.Command, args []string) error {
			return newAuth().Login(cmd.Context())
		},
	}

	return cmd
}

func logoutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "logout",
		RunE: func(cmd *cobra.Command, args []string) error {
			return newAuth().Logout()
		},
	}
	return cmd
}

func checkAuthenticated(ctx context.Context, cmd *cobra.Command, refresh bool) error {
	text := "It looks like you're not logged in. Do you want to log in now?"
	if refresh {
		text = "It seems that your authentication has expired. Would you like to re-authenticate now?"
	}
	model := tui.NewStandaloneQuestionModel(
		text,
		tui.MinimalKeyMap,
		tui.DefaultStyles,
	)
	finalModel, err := newProgram(cmd, model).Run()
	if err != nil {
		return errors.Wrap(err, "failed to prompt")
	}
	shouldLogIn := finalModel.(tui.StandaloneQuestionModel).Confirmed()

	if shouldLogIn {
		if loginErr := newAuth().Login(ctx); loginErr != nil {
			return errors.Wrap(loginErr, "failed to login")
		}
	}

	return nil
}

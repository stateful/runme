package cmd

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/stateful/runme/v3/internal/auth"
	"github.com/stateful/runme/v3/internal/tui"
)

func loginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "login",
		Hidden: true,
		Short:  "Log in to Runme (optional)",
		Long:   "Log in to Runme is not required for standalone functionality",
		RunE: func(cmd *cobra.Command, args []string) error {
			return newAuth().Login(cmd.Context())
		},
	}

	return cmd
}

func logoutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "logout",
		Hidden: true,
		Short:  "Log out from Runme",
		RunE: func(cmd *cobra.Command, args []string) error {
			return newAuth().Logout()
		},
	}
	return cmd
}

func tokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "token",
		Hidden: true,
		Short:  "Print runme API token",
		RunE: func(cmd *cobra.Command, args []string) error {
			var token string
			var err error
			if fInsecure {
				auth := newAuth()
				token, err = auth.GetToken(cmd.Context())
				if err != nil {

					if err := checkAuthenticated(cmd.Context(), cmd, auth, !recoverableWithLogin(err)); err != nil {
						return err
					}

					token, err = auth.GetToken(cmd.Context())
					if err != nil {
						return err
					}
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), token)
				return nil
			}
			return errors.New("To use this command, please add the --insecure parameter")
		},
	}

	return cmd
}

func checkAuthenticated(ctx context.Context, cmd *cobra.Command, auth auth.Authorizer, refresh bool) error {
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
		if loginErr := auth.Login(ctx); loginErr != nil {
			return errors.Wrap(loginErr, "failed to login")
		}
	}

	return nil
}

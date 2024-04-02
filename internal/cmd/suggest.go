package cmd

import (
	"context"
	"fmt"
	"log"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/stateful/runme/v3/internal/client/graphql"
	"github.com/stateful/runme/v3/internal/tui"
	"github.com/stateful/runme/v3/internal/tui/prompt"
)

func suggestCmd() *cobra.Command {
	var repoUser string

	cmd := &cobra.Command{
		Use:    "suggest DESCRIPTION",
		Hidden: true,
		Short:  "Suggest a branch name",
		Long: `Suggest a branch name for a description.

Remember to wrap the DESCRIPTION in double quotes as otherwise
it will be interpreted as multiple arguments.

Disclaimer: This uses AI, so the suggestions may be biased, wrong,
or just bad. Please use with discretion.
`,
		Deprecated: "Please note this command does not receive updates and will be removed in the future.",
		Args:       cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := graphql.ContextWithTrackInput(cmd.Context(), trackInputFromCmd(cmd, args))

			auth := newAuth()
			_, err := auth.GetToken(cmd.Context())
			if err != nil {
				if err := checkAuthenticated(ctx, cmd, auth, !recoverableWithLogin(err)); err != nil {
					return err
				}
			}
			client, err := graphql.New(graphqlEndpoint(), newAPIClient(cmd.Context(), auth))
			if err != nil {
				return err
			}

			// Run a query to get user's data exclusively in order to force authentication flow.
			err = runSuggestAndRetry(ctx, cmd, func(ctx context.Context) error {
				_, err := client.GetUser(ctx, false)
				return err
			})
			if err != nil {
				return err
			}

			var description string

			switch len(args) {
			case 0:
				description, err = promptForDescription(cmd)
				if err != nil {
					return err
				}
			case 1:
				description = args[0]
			}

			if len(description) == 0 {
				return errors.New("description cannot be empty")
			}

			model := tui.NewListModel(ctx, description, repoUser, client)
			m, err := newProgram(cmd, model).Start()
			if err != nil {
				return err
			}

			if mm, ok := m.(tui.ListModel); ok && mm.Confirmed() {
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "Great choice! Learn how to run your markdown files with https://runme.dev/\n\n")
			}

			return err
		},
	}

	cmd.Flags().StringVar(&repoUser, "repo-user", "", "Overwrite git user's email address used for user branch filtering.")

	return cmd
}

func promptForDescription(cmd *cobra.Command) (string, error) {
	model := tui.NewStandaloneInputModel(prompt.InputParams{Label: "Enter a description:"}, tui.MinimalKeyMap, tui.DefaultStyles)
	finalModel, err := newProgram(cmd, model).Run()
	if err != nil {
		return "", err
	}
	val, ok := finalModel.(tui.StandaloneInputModel).Value()
	if !ok {
		return "", errors.New("canceled")
	}
	return val, nil
}

func runSuggestAndRetry(ctx context.Context, _ *cobra.Command, runF runFunc) error {
	err := runF(ctx)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, graphql.ErrNoData):
		return err
	case !recoverableWithLogin(err):
		log.Print("Unexpected error occurred. Please try again later or report the issue at https://github.com/stateful/runme or our Discord at https://discord.gg/stateful")
		return errors.Wrap(err, "failed to run command")
	default:
		// continue as it likely was an auth problem
	}

	return runF(ctx)
}

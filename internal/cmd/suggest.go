package cmd

import (
	"context"
	"log"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/stateful/runme/internal/client/graphql"
	"github.com/stateful/runme/internal/tui"
)

func suggestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "suggest",
		Short: "Use our suggestion engine to give contextual advice",
	}

	return cmd
}

func branchCmd() *cobra.Command {
	var repoUser string

	cmd := &cobra.Command{
		Use:   "branch DESCRIPTION",
		Short: "Suggest a branch name",
		Long: `Suggest a branch name for a description.

Remember to wrap the DESCRIPTION in double quotes as otherwise
it will be interpreted as multiple arguments.

NB: This uses machine learning, so the suggestions may be biased, wrong, or just
bad. Please use with discretion.
`,
		Args: cobra.MaximumNArgs(1),
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
			return newProgram(cmd, model).Start()
		},
	}

	cmd.Flags().StringVar(&repoUser, "repo-user", "", "Overwrite git user's email address used for user branch filtering.")

	return cmd
}

func promptForDescription(cmd *cobra.Command) (string, error) {
	model := tui.NewStandaloneInputModel("Enter a description:", tui.MinimalKeyMap, tui.DefaultStyles)
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

func runSuggestAndRetry(ctx context.Context, cmd *cobra.Command, runF runFunc) error {
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

package cmd

import (
	"context"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/stateful/runme/client/graphql"

	"github.com/stateful/runme/internal/renderer"
	sugg "github.com/stateful/runme/internal/suggestions"
)

func suggestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "suggest",
		Short:  "Use our suggestion engine to give contextual advice",
		Hidden: true,
	}

	branchCmd := branchCmd()

	cmd.AddCommand(branchCmd)
	cmd.RunE = branchCmd.RunE

	return cmd
}

func branchCmd() *cobra.Command {
	var repoUser string

	cmd := &cobra.Command{
		Use:   "branch DESCRIPTION",
		Short: "Suggest a branchname.",
		Long: `Suggest a branchname for a description.

Remember to wrap the DESCRIPTION in double quotes as otherwise
it will be interpreted as multiple arguments.

NB: This uses machine learning, so the suggestions may be biased, wrong, or just
bad. Please use with discretion.
`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := graphql.ContextWithTrackInput(cmd.Context(), trackInputFromCmd(cmd, args))

			client, err := graphql.New(graphqlEndpoint(), newAPIClient(cmd.Context()))
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

			model := sugg.NewListModel(ctx, description, repoUser, client)
			return newProgram(cmd, model).Start()
		},
	}

	cmd.Flags().StringVar(&repoUser, "repo-user", "", "Overwrite git user's email address used for user branch filtering.")

	return cmd
}

func promptForDescription(cmd *cobra.Command) (string, error) {
	model := renderer.NewStandaloneInputModel("Enter a description:", renderer.MinimalKeyMap, renderer.DefaultStyles)
	finalModel, err := newProgram(cmd, model).StartReturningModel()
	if err != nil {
		return "", err
	}
	val, ok := finalModel.(renderer.StandaloneInputModel).Value()
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
		return errors.Wrap(err, "failed to run command")
	default:
		// continue as it likely was an auth problem
	}

	if err := checkAuthenticated(ctx, cmd); err != nil {
		return err
	}

	return runF(ctx)
}

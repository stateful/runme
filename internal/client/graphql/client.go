package graphql

import (
	"context"
	"net/http"

	"github.com/Khan/genqlient/graphql"
	"github.com/stateful/runme/v3/internal/client/graphql/query"
)

//go:generate genqlient

type Client struct {
	graphql.Client
}

func New(endpoint string, httpClient *http.Client) (*Client, error) {
	return &Client{
		Client: graphql.NewClient(endpoint, httpClient),
	}, nil
}

func (c *Client) GetUser(ctx context.Context, withAnnotation bool) (result query.GetUserUser, _ error) {
	trackInput := trackInputFromContext(ctx)
	resp, err := query.GetUser(ctx, c.Client, withAnnotation, trackInput)
	if err := NewAPIError(err, resp.Track.Errors); err != nil {
		return result, err
	}
	return resp.User, nil
}

func (c *Client) GetSuggestedBranch(ctx context.Context, q query.SuggestedBranchInput) (result []string, _ error) {
	trackInput := trackInputFromContext(ctx)
	resp, err := query.GetSuggestedBranch(ctx, c.Client, q, trackInput)
	if err != nil {
		return nil, NewAPIError(err)
	}

	var names []string
	for _, item := range resp.SuggestedBranchnames.Data {
		names = append(names, item.Name)
	}

	return names, err
}

type ctxKey struct{}

var trackInputKey = &ctxKey{}

func ContextWithTrackInput(ctx context.Context, input query.TrackInput) context.Context {
	return context.WithValue(ctx, trackInputKey, input)
}

func trackInputFromContext(ctx context.Context) query.TrackInput {
	inpt, _ := ctx.Value(trackInputKey).(query.TrackInput)
	return inpt
}

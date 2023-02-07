package graphql

import (
	"context"
	"net/http"

	"github.com/Khan/genqlient/graphql"
	"github.com/stateful/runme/client/graphql/query"
)

type Client struct {
	graphql.Client
}

func New(endpoint string, httpClient *http.Client) (*Client, error) {
	return &Client{
		Client: graphql.NewClient(endpoint, httpClient),
	}, nil
}

func (c *Client) GetStandup(ctx context.Context, date *string, upcoming bool) (result query.Standup, err error) {
	trackInput := trackInputFromContext(ctx)
	resp, err := query.GetStandup(ctx, c.Client, date, upcoming, trackInput)
	if err := NewAPIError(err, resp.Track.Errors); err != nil {
		return result, err
	}
	if resp.Standup.Date == "" {
		return result, ErrNoData
	}
	return resp.Standup.Standup, nil
}

func (c *Client) GetStandups(ctx context.Context, upcoming bool, pageSize int) (result []query.ListStandup, err error) {
	trackInput := trackInputFromContext(ctx)
	resp, err := query.GetStandups(ctx, c.Client, upcoming, pageSize, trackInput)
	if err := NewAPIError(err, resp.Track.Errors); err != nil {
		return result, err
	}
	if len(resp.Standups.Data) == 0 {
		return result, ErrNoData
	}
	return resp.Standups.Data, nil
}

func (c *Client) GetDay(ctx context.Context, date *string) (result query.Day, err error) {
	trackInput := trackInputFromContext(ctx)
	resp, err := query.GetDay(ctx, c.Client, date, trackInput)
	if err := NewAPIError(err, resp.Track.Errors); err != nil {
		return result, err
	}
	if resp.Day.Date == "" {
		return result, ErrNoData
	}
	return resp.Day.Day, nil
}

type GetDaysArgs struct {
	StartDate     *string `json:"startDate,omitempty"`
	EndDate       *string `json:"endDate,omitempty"`
	PageSize      int     `json:"pageSize,omitempty"`
	NextPageToken string  `json:"nextPageToken,omitempty"`
}

func (c *Client) GetDays(ctx context.Context, args GetDaysArgs) (result []query.Day, nextPageToken string, err error) {
	trackInput := trackInputFromContext(ctx)
	resp, err := query.GetDays(ctx, c.Client, args.StartDate, args.EndDate, args.PageSize, args.NextPageToken, trackInput)
	if err := NewAPIError(err, resp.Track.Errors); err != nil {
		return result, nextPageToken, err
	}
	if len(resp.Days.Data) == 0 {
		return result, nextPageToken, ErrNoData
	}
	return resp.Days.Data, resp.Days.NextPageToken, nil
}

func (c *Client) GetUser(ctx context.Context, withAnnotation bool) (result query.GetUserUser, _ error) {
	trackInput := trackInputFromContext(ctx)
	resp, err := query.GetUser(ctx, c.Client, withAnnotation, trackInput)
	if err := NewAPIError(err, resp.Track.Errors); err != nil {
		return result, err
	}
	return resp.User, nil
}

func (c *Client) AddNote(ctx context.Context, input query.CreateUserAnnotationInput) error {
	trackInput := trackInputFromContext(ctx)
	resp, err := query.AddNote(ctx, c.Client, input, trackInput)
	return NewAPIError(err, resp.CreateUserAnnotation.Errors, resp.Track.Errors)
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

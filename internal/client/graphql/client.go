package graphql

import (
	"net/http"

	"github.com/Khan/genqlient/graphql"
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

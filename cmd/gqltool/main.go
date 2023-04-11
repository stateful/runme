package main

import (
	"context"
	"flag"
	"fmt"
	"log" // revive:disable-line
	"os"
	"path/filepath"

	"github.com/stateful/runme/internal/auth"
	"github.com/stateful/runme/internal/client"
	"github.com/stateful/runme/internal/client/graphql"
	"golang.org/x/oauth2"
)

var (
	apiURL   = flag.String("api-url", "https://api.stateful.com", "The API base address")
	tokenDir = flag.String("token-dir", getDefaultConfigHome(), "The directory with tokens")
)

func init() {
	flag.Parse()
}

func main() {
	httpClient := client.NewHTTPClient(nil, client.WithTokenGetter(func() (string, error) {
		a := auth.New(oauth2.Config{}, *apiURL, &auth.DiskStorage{Location: *tokenDir})
		return a.GetToken(context.Background())
	}))
	client, err := graphql.New(*apiURL+"/graphql", httpClient)
	if err != nil {
		log.Fatal(err)
	}
	result, err := client.IntrospectionQuery(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	_, _ = fmt.Fprintf(os.Stdout, "%s", result)
}

func getDefaultConfigHome() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		panic(err)
	}
	_, fErr := os.Stat(dir)
	if os.IsNotExist(fErr) {
		mkdErr := os.MkdirAll(dir, os.ModePerm)
		if mkdErr != nil {
			dir = os.TempDir()
		}
	}
	return filepath.Join(dir, "stateful")
}

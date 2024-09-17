package main

import (
	"context"
	"flag"
	"fmt"
	"log" // revive:disable-line
	"os"

	"github.com/stateful/runme/v3/internal/client"
	"github.com/stateful/runme/v3/internal/client/graphql"
)

var apiURL = flag.String("api-url", "http://localhost:4000", "The API base address")

func init() {
	flag.Parse()
}

func main() {
	httpClient := client.NewHTTPClient(nil)
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

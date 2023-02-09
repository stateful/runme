# GraphQL Schema

This package contains schema of the Stateful GraphQL API.

## Generate

The process of generating Go models from the schema works as follows:

1. Download schema represented as an Introspection Query result: `go run ./cmd/gqltool/main.go --api-url=[YOUR STAGING URL] > ./internal/client/graphql/schema/introspection_query_result.json`
1. Convert it to GraphQL Schema Definition Language using `graphql-js`:
   ```
   $ pushd ./internal/client/graphql/schema
   $ npm run convert
   $ popd
   ```
1. Generate Go stubs: `make generate`

## Resources

[1] [Three ways to represent your GraphQL schema](https://www.apollographql.com/blog/backend/schema-design/three-ways-to-represent-your-graphql-schema/)

package owl

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/testutil"
	"github.com/stretchr/testify/require"
)

func Test_Graph(t *testing.T) {
	t.Parallel()

	t.Run("introspect schema", func(t *testing.T) {
		result := graphql.Do(graphql.Params{
			Schema:        Schema,
			RequestString: testutil.IntrospectionQuery,
		})
		require.False(t, result.HasErrors())

		b, err := json.MarshalIndent(result, "", " ")
		require.NoError(t, err)

		// err = os.WriteFile("../../schema.json", b, 0o644)
		// require.NoError(t, err)

		require.NotNil(t, b)
	})

	t.Run("query environment", func(t *testing.T) {
		var vars map[string]interface{}
		err := json.Unmarshal([]byte(`{"load_0":[{"created":"2024-03-02T13:25:01.270468-05:00","key":"GOPATH","operation":null,"required":false,"spec":{"checked":false,"name":"Opaque"},"value":{"resolved":"/Users/sourishkrout/go","status":""}},{"created":"2024-03-02T13:25:01.270469-05:00","key":"HOME","operation":null,"required":false,"spec":{"checked":false,"name":"Opaque"},"value":{"resolved":"/Users/sourishkrout","status":""}},{"created":"2024-03-02T13:25:01.270471-05:00","key":"HOMEBREW_REPOSITORY","operation":null,"required":false,"spec":{"checked":false,"name":"Opaque"},"value":{"resolved":"/opt/homebrew","status":""}}]}`), &vars)
		require.NoError(t, err)

		result := graphql.Do(graphql.Params{
			Schema: Schema,
			RequestString: `query ResolveOwlSnapshot($insecure: Boolean = false, $load_0: [VariableInput]!) {
  environment {
    load(vars: $load_0, hasSpecs: false) {
      location
      snapshot(insecure: $insecure) {
        key
        value {
          original
          resolved
          status
        }
        spec {
          name
        }
        required
        created
        updated
      }
    }
  }
}`,
			VariableValues: vars,
		})
		require.False(t, result.HasErrors())

		b, err := json.MarshalIndent(result, "", " ")
		require.NoError(t, err)
		fmt.Println(string(b))
		require.NotNil(t, b)

		snapshot := result.Data.(map[string]interface{})["environment"].(map[string]interface{})["load"].(map[string]interface{})["snapshot"]
		require.NoError(t, err)
		require.Len(t, snapshot, 3)
	})

	t.Run("query complex environment", func(t *testing.T) {
		jsonVars := `{"delete_2":[{"created":"2024-03-08T11:09:07.125844-05:00","key":"TEST_OLD_UNSET","operation":null,"required":false,"spec":{"checked":false,"name":"Opaque"},"value":{"status":""}}],"insecure":true,"load_0":[{"created":"2024-03-08T11:09:07.106634-05:00","key":"TEST_OLD","operation":null,"required":false,"spec":{"checked":false,"name":"Opaque"},"value":{"original":"value1","status":""}},{"created":"2024-03-08T11:09:07.106634-05:00","key":"TEST_OLD_CHANGED","operation":null,"required":false,"spec":{"checked":false,"name":"Opaque"},"value":{"original":"value1","status":""}},{"created":"2024-03-08T11:09:07.106635-05:00","key":"TEST_OLD_UNSET","operation":null,"required":false,"spec":{"checked":false,"name":"Opaque"},"value":{"original":"value1","status":""}}],"update_1":[{"created":"2024-03-08T11:09:07.125842-05:00","key":"TEST_OLD_CHANGED","operation":null,"required":false,"spec":{"checked":false,"name":"Opaque"},"value":{"original":"value2","status":""}},{"created":"2024-03-08T11:09:07.125843-05:00","key":"TEST_NEW","operation":null,"required":false,"spec":{"checked":false,"name":"Opaque"},"value":{"original":"value2","status":""}}]}`
		query := `query ResolveOwlSnapshot($insecure: Boolean = false, $load_0: [VariableInput]!, $update_1: [VariableInput]!, $delete_2: [VariableInput]!) {
  environment {
    load(vars: $load_0, location: ".env", hasSpecs: false) {
      location
      update(vars: $update_1, location: "cell#01HQQZ60MHZEDTXWG70PMXZNHH", hasSpecs: false) {
        location
        delete(vars: $delete_2, location: "cell#01HQQZ7AYPP87D1MB6D06G9BTX", hasSpecs: false) {
          location
          validate {
            Opaque(insecure: $insecure, keys: ["TEST_OLD_CHANGED", "TEST_OLD_UNSET", "TEST_OLD", "TEST_NEW"]) {
              spec
              sensitive
              mask
              errors
              done {
                snapshot(insecure: $insecure) {
                  key
                  value {
                    original
                    resolved
                    status
                  }
                  spec {
                    name
                  }
                  required
                  created
                  updated
                }
              }
            }
          }
        }
      }
    }
  }
}`
		var vars map[string]interface{}
		err := json.Unmarshal([]byte(jsonVars), &vars)
		require.NoError(t, err)

		result := graphql.Do(graphql.Params{
			Schema:         Schema,
			RequestString:  query,
			VariableValues: vars,
		})

		fmt.Println(result.Errors)
		require.False(t, result.HasErrors())

		b, _ := json.MarshalIndent(result, "", " ")
		fmt.Println(string(b))
	})

	t.Run("query specs list", func(t *testing.T) {
		result := graphql.Do(graphql.Params{
			Schema:        Schema,
			RequestString: `query { specs { list { name } } }`,
		})
		require.False(t, result.HasErrors())

		b, err := json.MarshalIndent(result, "", " ")
		require.NoError(t, err)
		// fmt.Println(string(b))

		require.NotNil(t, b)
	})

	t.Run("query specs for real", func(t *testing.T) {
		var vars map[string]interface{}
		err := json.Unmarshal([]byte(`{"load_0":[{"created":"2024-03-02T13:25:01.270468-05:00","key":"GOPATH","operation":null,"required":false,"spec":{"checked":false,"name":"Opaque"},"value":{"resolved":"/Users/sourishkrout/go","status":""}},{"created":"2024-03-02T13:25:01.270469-05:00","key":"HOME","operation":null,"required":true,"spec":{"checked":false,"name":"Secret"},"value":{"resolved":"/Users/sourishkrout","status":""}},{"created":"2024-03-02T13:25:01.270471-05:00","key":"HOMEBREW_REPOSITORY","operation":null,"required":false,"spec":{"checked":false,"name":"Plain"},"value":{"resolved":"/opt/homebrew","status":""}}]}`), &vars)
		require.NoError(t, err)

		result := graphql.Do(graphql.Params{
			Schema: Schema,
			RequestString: `query ResolveOwlSnapshot(
  $insecure: Boolean = false
  $load_0: [VariableInput]!
) {
  environment {
    load(vars: $load_0, hasSpecs: false) {
      validate {
        Opaque(keys: ["GOPATH"]) {
          spec
          sensitive
          mask
          errors
          Secret(keys: ["HOME"]) {
            spec
            sensitive
            mask
            errors
            Plain(keys: ["HOMEBREW_REPOSITORY"]) {
              spec
              sensitive
              mask
              errors
              done {
                snapshot(insecure: $insecure) {
                  key
                  value {
                    original
                    resolved
                    status
                  }
                  spec {
                    name
                  }
                  required
                  created
                  updated
                }
              }
            }
          }
        }
      }
    }
  }
}`,
			VariableValues: vars,
		})
		require.False(t, result.HasErrors())
		fmt.Println(result.Errors)

		b, err := json.MarshalIndent(result, "", " ")
		require.NoError(t, err)
		fmt.Println(string(b))
		require.NotNil(t, b)
	})
}

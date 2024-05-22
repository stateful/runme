package owl

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func Test_Graph(t *testing.T) {
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
}

func Test_QuerySpecs(t *testing.T) {
	t.Run("query list of specs", func(t *testing.T) {
		result := graphql.Do(graphql.Params{
			Schema:        Schema,
			RequestString: `query { specs { list { name } } }`,
		})
		require.False(t, result.HasErrors())

		b, err := json.MarshalIndent(result, "", " ")
		require.NoError(t, err)
		_, _ = fmt.Println(string(b))

		require.NotNil(t, b)
	})
}

type fileTestCase struct {
	name string
	file string
	pre  func(t *testing.T, vars map[string]interface{}, query *[]byte)
	post func(t *testing.T, result *graphql.Result)
}

type fileTestCases []fileTestCase

func (testCases fileTestCases) runAll(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fileName := tc.file
			if fileName == "" {
				fileName = tc.name
			}

			fileName = strings.ReplaceAll(strings.ReplaceAll(strings.ToLower((fileName)), " ", "_"), "-", "_")

			buf, err := os.ReadFile(filepath.Join("testdata", "graph", fileName+".json"))
			require.NoError(t, err)
			var vars map[string]interface{}
			err = json.Unmarshal(buf, &vars)
			require.NoError(t, err)

			query, err := os.ReadFile(filepath.Join("testdata", "graph", fileName+".graphql"))
			require.NoError(t, err)

			if tc.pre != nil {
				tc.pre(t, vars, &query)
			}

			result := graphql.Do(graphql.Params{
				Schema:         Schema,
				RequestString:  string(query),
				VariableValues: vars,
			})
			require.False(t, result.HasErrors())

			b, err := json.MarshalIndent(result, "", " ")
			require.NoError(t, err)
			_, _ = fmt.Println(string(b))
			require.NotNil(t, b)

			if tc.post == nil {
				return
			}

			tc.post(t, result)
		})
	}
}

func Test_ResolveEnv(t *testing.T) {
	t.Parallel()

	testCases := fileTestCases{
		{
			name: "Query simple env",
			post: func(t *testing.T, result *graphql.Result) {
				snapshot, err := extractDataKey(result.Data, "snapshot")
				require.NoError(t, err)
				require.Len(t, snapshot, 3)
			},
		},
		{
			name: "Query complex env",
			post: func(t *testing.T, result *graphql.Result) {
				val, err := extractDataKey(result.Data, "snapshot")
				require.NoError(t, err)

				j, err := json.MarshalIndent(val, "", " ")
				require.NoError(t, err)

				var snapshot SetVarItems
				err = json.Unmarshal(j, &snapshot)
				require.NoError(t, err)

				snapshot.sortbyKey()

				for _, v := range snapshot {
					if v.Var.Key != "NAME" {
						continue
					}
					require.EqualValues(t, ".env", v.Var.Origin)
					require.EqualValues(t, "Loon", v.Value.Resolved)
					require.EqualValues(t, "LITERAL", v.Value.Status)
					require.EqualValues(t, "Plain", v.Spec.Name)
				}
				b, err := yaml.Marshal(snapshot)
				require.NoError(t, err)
				_, _ = fmt.Println(string(b))
				require.NotNil(t, b)
			},
		},
		{
			name: "Env without specs",
			post: func(t *testing.T, result *graphql.Result) {
				render, err := extractDataKey(result.Data, "render")
				require.NoError(t, err)
				require.NotNil(t, render)

				b, err := yaml.Marshal(render)
				// b, err := json.MarshalIndent(result, "", " ")
				require.NoError(t, err)
				_, _ = fmt.Println(string(b))
				require.NotNil(t, b)
			},
		},
	}

	testCases.runAll(t)
}

func Test_Graph_Update(t *testing.T) {
	testCases := fileTestCases{
		{
			name: "Store update",
			post: func(t *testing.T, result *graphql.Result) {
				render, err := extractDataKey(result.Data, "render")
				require.NoError(t, err)
				require.NotNil(t, render)
			},
		},
	}

	testCases.runAll(t)
}

func Test_Graph_Required(t *testing.T) {
	testCases := fileTestCases{
		{
			name: "Validate simple env",
			post: func(t *testing.T, result *graphql.Result) {
				validate, err := extractDataKey(result.Data, "validate")
				require.NoError(t, err)
				require.NotNil(t, validate)

				b, err := yaml.Marshal(validate)
				// b, err := json.MarshalIndent(result, "", " ")
				require.NoError(t, err)
				_, _ = fmt.Println(string(b))
				require.NotNil(t, b)
			},
		},
	}

	testCases.runAll(t)
}

func Test_Graph_Reconcile(t *testing.T) {
	testCases := fileTestCases{
		{
			name: "Reconcile operationless",
			post: func(t *testing.T, result *graphql.Result) {
				validate, err := extractDataKey(result.Data, "validate")
				require.NoError(t, err)
				require.NotNil(t, validate)

				b, err := yaml.Marshal(validate)
				// b, err := json.MarshalIndent(result, "", " ")
				require.NoError(t, err)
				_, _ = fmt.Println(string(b))
				require.NotNil(t, b)
			},
		},
	}

	testCases.runAll(t)
}

func Test_Graph_SensitiveKeys(t *testing.T) {
	testCases := fileTestCases{
		{
			name: "Sensitive keys",
			post: func(t *testing.T, result *graphql.Result) {
				render, err := extractDataKey(result.Data, "render")
				require.NoError(t, err)
				require.NotNil(t, render)

				b, err := yaml.Marshal(render)
				// b, err := json.MarshalIndent(result, "", " ")
				require.NoError(t, err)
				_, _ = fmt.Println(string(b))
				require.NotNil(t, b)
			},
		},
	}

	testCases.runAll(t)
}

func Test_Graph_Get(t *testing.T) {
	testCases := fileTestCases{
		{
			name: "InsecureGet",
			post: func(t *testing.T, result *graphql.Result) {
				render, err := extractDataKey(result.Data, "render")
				require.NoError(t, err)
				require.NotNil(t, render)

				b, err := yaml.Marshal(render)
				// b, err := json.MarshalIndent(result, "", " ")
				require.NoError(t, err)
				_, _ = fmt.Println(string(b))
				require.NotNil(t, b)
				assert.Contains(t, string(b), "/opt/homebrew/share/google-cloud-sdk/bin")
			},
		},
	}

	testCases.runAll(t)
}

func Test_Graph_DotEnv(t *testing.T) {
	testCases := fileTestCases{
		{
			name: "Without prefix",
			file: "dotenv",
			post: func(t *testing.T, result *graphql.Result) {
				dotenv, err := extractDataKey(result.Data, "dotenv")
				require.NoError(t, err)
				require.NotNil(t, dotenv)

				dotenvStr, ok := dotenv.(string)
				require.True(t, ok)
				require.False(t, strings.HasPrefix(dotenvStr, "VITE_APP"))

				_, _ = fmt.Println(dotenvStr)
			},
		},
		{
			name: "VITE_APP prefix",
			file: "dotenv",
			pre: func(t *testing.T, vars map[string]interface{}, query *[]byte) {
				prefix := "VITE_APP_"
				vars["prefix"] = prefix
				vars["insecure"] = false
			},
			post: func(t *testing.T, result *graphql.Result) {
				dotenv, err := extractDataKey(result.Data, "dotenv")
				require.NoError(t, err)
				require.NotNil(t, dotenv)

				prefix := "VITE_APP_"
				dotenvStr, ok := dotenv.(string)
				require.True(t, ok)
				require.True(t, strings.HasPrefix(dotenvStr, prefix))

				nlines := len(strings.Split(dotenvStr, "\n"))
				require.EqualValues(t, 7, nlines)
			},
		},
		{
			name: "With insecure",
			file: "dotenv",
			pre: func(t *testing.T, vars map[string]interface{}, query *[]byte) {
				vars["insecure"] = true
			},
			post: func(t *testing.T, result *graphql.Result) {
				dotenv, err := extractDataKey(result.Data, "dotenv")
				require.NoError(t, err)
				require.NotNil(t, dotenv)

				dotenvStr, ok := dotenv.(string)
				require.True(t, ok)
				nlines := len(strings.Split(dotenvStr, "\n"))
				require.EqualValues(t, 60, nlines)
			},
		},
	}

	testCases.runAll(t)
}

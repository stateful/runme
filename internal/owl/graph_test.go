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
		fmt.Println(string(b))

		require.NotNil(t, b)
	})
}

type fileTestCase struct {
	name         string
	assertResult func(t *testing.T, result *graphql.Result)
}

type fileTestCases []fileTestCase

func (testCases fileTestCases) runAll(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			buf, err := os.ReadFile(filepath.Join("testdata", "graph", tc.name+".json"))
			require.NoError(t, err)
			var vars map[string]interface{}
			err = json.Unmarshal(buf, &vars)
			require.NoError(t, err)

			query, err := os.ReadFile(filepath.Join("testdata", "graph", tc.name+".graphql"))
			require.NoError(t, err)
			result := graphql.Do(graphql.Params{
				Schema:         Schema,
				RequestString:  string(query),
				VariableValues: vars,
			})
			require.False(t, result.HasErrors())

			b, err := json.MarshalIndent(result, "", " ")
			require.NoError(t, err)
			fmt.Println(string(b))
			require.NotNil(t, b)

			tc.assertResult(t, result)
		})
	}
}

func Test_ResolveEnv(t *testing.T) {
	t.Parallel()

	testCases := fileTestCases{
		{
			name: "query_simple_env",
			assertResult: func(t *testing.T, result *graphql.Result) {
				snapshot, err := extractDataKey(result.Data, "snapshot")
				require.NoError(t, err)
				require.Len(t, snapshot, 3)
			},
		},
		{
			name: "query_complex_env",
			assertResult: func(t *testing.T, result *graphql.Result) {
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
				fmt.Println(string(b))
				require.NotNil(t, b)
			},
		},
		{
			name: "env_without_specs",
			assertResult: func(t *testing.T, result *graphql.Result) {
				render, err := extractDataKey(result.Data, "render")
				require.NoError(t, err)
				require.NotNil(t, render)

				b, err := yaml.Marshal(render)
				// b, err := json.MarshalIndent(result, "", " ")
				require.NoError(t, err)
				fmt.Println(string(b))
				require.NotNil(t, b)
			},
		},
	}

	testCases.runAll(t)
}

func Test_Graph_Update(t *testing.T) {
	testCases := fileTestCases{
		{
			name: "store_update",
			assertResult: func(t *testing.T, result *graphql.Result) {
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
			name: "validate_simple_env",
			assertResult: func(t *testing.T, result *graphql.Result) {
				validate, err := extractDataKey(result.Data, "validate")
				require.NoError(t, err)
				require.NotNil(t, validate)

				b, err := yaml.Marshal(validate)
				// b, err := json.MarshalIndent(result, "", " ")
				require.NoError(t, err)
				fmt.Println(string(b))
				require.NotNil(t, b)
			},
		},
	}

	testCases.runAll(t)
}

func Test_Graph_Reconcile(t *testing.T) {
	testCases := fileTestCases{
		{
			name: "reconcile_operationless",
			assertResult: func(t *testing.T, result *graphql.Result) {
				validate, err := extractDataKey(result.Data, "validate")
				require.NoError(t, err)
				require.NotNil(t, validate)

				b, err := yaml.Marshal(validate)
				// b, err := json.MarshalIndent(result, "", " ")
				require.NoError(t, err)
				fmt.Println(string(b))
				require.NotNil(t, b)
			},
		},
	}

	testCases.runAll(t)
}

func Test_Graph_Sensitive(t *testing.T) {
	testCases := fileTestCases{
		{
			name: "sensitive_keys",
			assertResult: func(t *testing.T, result *graphql.Result) {
				render, err := extractDataKey(result.Data, "render")
				require.NoError(t, err)
				require.NotNil(t, render)

				b, err := yaml.Marshal(render)
				// b, err := json.MarshalIndent(result, "", " ")
				require.NoError(t, err)
				fmt.Println(string(b))
				require.NotNil(t, b)
			},
		},
	}

	testCases.runAll(t)
}

func Test_Graph_DotEnv(t *testing.T) {
	var vars map[string]interface{}
	err := json.Unmarshal([]byte(`{"load_0":[{"value":{"original":"fi=00:mi=00:mh=00:ln=01;36:or=01;31:di=01;34:ow=04;01;34:st=34:tw=04;34:pi=01;33:so=01;33:do=01;33:bd=01;33:cd=01;33:su=01;35:sg=01;35:ca=01;35:ex=01;32","status":""},"var":{"created":"2024-03-10T20:48:58.072091-04:00","key":"LS_COLORS","operation":null}},{"value":{"original":"0","status":""},"var":{"created":"2024-03-10T20:48:58.072092-04:00","key":"MallocNanoZone","operation":null}},{"value":{"original":"0","status":""},"var":{"created":"2024-03-10T20:48:58.072102-04:00","key":"SHLVL","operation":null}},{"value":{"original":"/var/folders/c3/5r0t1nzs7sbfpxjgbc6n3ss40000gn/T/","status":""},"var":{"created":"2024-03-10T20:48:58.072104-04:00","key":"TMPDIR","operation":null}},{"value":{"original":"/Users/sourishkrout/.wasmtime","status":""},"var":{"created":"2024-03-10T20:48:58.072111-04:00","key":"WASMTIME_HOME","operation":null}},{"value":{"original":"1","status":""},"var":{"created":"2024-03-10T20:48:58.072115-04:00","key":"APPLICATION_INSIGHTS_NO_DIAGNOSTIC_CHANNEL","operation":null}},{"value":{"original":"/Users/sourishkrout/go","status":""},"var":{"created":"2024-03-10T20:48:58.072081-04:00","key":"GOPATH","operation":null}},{"value":{"original":"achristian","status":""},"var":{"created":"2024-03-10T20:48:58.072087-04:00","key":"KRAFTCLOUD_USER","operation":null}},{"value":{"original":"/private/tmp/com.apple.launchd.WJncT7ZrHW/Listeners","status":""},"var":{"created":"2024-03-10T20:48:58.072103-04:00","key":"SSH_AUTH_SOCK","operation":null}},{"value":{"original":"sourishkrout","status":""},"var":{"created":"2024-03-10T20:48:58.072107-04:00","key":"USER","operation":null}},{"value":{"original":"True","status":""},"var":{"created":"2024-03-10T20:48:58.072107-04:00","key":"USE_GKE_GCLOUD_AUTH_PLUGIN","operation":null}},{"value":{"original":"/","status":""},"var":{"created":"2024-03-10T20:48:58.072109-04:00","key":"VSCODE_CWD","operation":null}},{"value":{"original":"less","status":""},"var":{"created":"2024-03-10T20:48:58.072095-04:00","key":"PAGER","operation":null}},{"value":{"original":"/Users/sourishkrout/Library/Application Support/Code/1.87-main.sock","status":""},"var":{"created":"2024-03-10T20:48:58.07211-04:00","key":"VSCODE_IPC_HOOK","operation":null}},{"value":{"original":"{\"locale\":\"en-us\",\"osLocale\":\"en-us\",\"availableLanguages\":{},\"_languagePackSupport\":true}","status":""},"var":{"created":"2024-03-10T20:48:58.07211-04:00","key":"VSCODE_NLS_CONFIG","operation":null}},{"value":{"original":"com.microsoft.VSCode","status":""},"var":{"created":"2024-03-10T20:48:58.072113-04:00","key":"__CFBundleIdentifier","operation":null}},{"value":{"original":"0x1F5:0x0:0x0","status":""},"var":{"created":"2024-03-10T20:48:58.072114-04:00","key":"__CF_USER_TEXT_ENCODING","operation":null}},{"value":{"original":"/opt/homebrew","status":""},"var":{"created":"2024-03-10T20:48:58.072082-04:00","key":"HOMEBREW_PREFIX","operation":null}},{"value":{"original":"/opt/homebrew/share/info:","status":""},"var":{"created":"2024-03-10T20:48:58.072085-04:00","key":"INFOPATH","operation":null}},{"value":{"original":"sourishkrout","status":""},"var":{"created":"2024-03-10T20:48:58.072089-04:00","key":"LOGNAME","operation":null}},{"value":{"original":"0x0","status":""},"var":{"created":"2024-03-10T20:48:58.072112-04:00","key":"XPC_FLAGS","operation":null}},{"value":{"original":"/opt/homebrew/opt/asdf/libexec","status":""},"var":{"created":"2024-03-10T20:48:58.072076-04:00","key":"ASDF_DIR","operation":null}},{"value":{"original":"/Users/sourishkrout","status":""},"var":{"created":"2024-03-10T20:48:58.072081-04:00","key":"HOME","operation":null}},{"value":{"original":"ExGxDxDxCxDxDxFxFxexEx","status":""},"var":{"created":"2024-03-10T20:48:58.072089-04:00","key":"LSCOLORS","operation":null}},{"value":{"original":"/","status":""},"var":{"created":"2024-03-10T20:48:58.072096-04:00","key":"PWD","operation":null}},{"value":{"original":"extensionHost","status":""},"var":{"created":"2024-03-10T20:48:58.072108-04:00","key":"VSCODE_CRASH_REPORTER_PROCESS_TYPE","operation":null}},{"value":{"original":"true","status":""},"var":{"created":"2024-03-10T20:48:58.072109-04:00","key":"VSCODE_HANDLES_UNCAUGHT_ERRORS","operation":null}},{"value":{"original":"89716","status":""},"var":{"created":"2024-03-10T20:48:58.072111-04:00","key":"VSCODE_PID","operation":null}},{"value":{"original":"/Applications/Visual Studio Code.app/Contents/MacOS/Electron","status":""},"var":{"created":"2024-03-10T20:48:58.072113-04:00","key":"_","operation":null}},{"value":{"original":"unix2003","status":""},"var":{"created":"2024-03-10T20:48:58.07208-04:00","key":"COMMAND_MODE","operation":null}},{"value":{"original":"-iRFXMx4","status":""},"var":{"created":"2024-03-10T20:48:58.072088-04:00","key":"LESS","operation":null}},{"value":{"original":"1","status":""},"var":{"created":"2024-03-10T20:48:58.072115-04:00","key":"ELECTRON_RUN_AS_NODE","operation":null}},{"value":{"original":"/opt/homebrew/share/man:/usr/share/man:/usr/local/share/man:/Users/sourishkrout/.cache/zsh4humans/v5/fzf/man:","status":""},"var":{"created":"2024-03-10T20:48:58.072092-04:00","key":"MANPATH","operation":null}},{"value":{"original":"/","status":""},"var":{"created":"2024-03-10T20:48:58.072093-04:00","key":"OLDPWD","operation":null}},{"value":{"original":"/Users/sourishkrout/.terminfo","status":""},"var":{"created":"2024-03-10T20:48:58.072104-04:00","key":"TERMINFO","operation":null}},{"value":{"original":"application.com.microsoft.VSCode.251091548.251091554","status":""},"var":{"created":"2024-03-10T20:48:58.072112-04:00","key":"XPC_SERVICE_NAME","operation":null}},{"value":{"original":"undefined","status":""},"var":{"created":"2024-03-10T20:48:58.072094-04:00","key":"ORIGINAL_XDG_CURRENT_DESKTOP","operation":null}},{"value":{"original":"/bin/zsh","status":""},"var":{"created":"2024-03-10T20:48:58.072102-04:00","key":"SHELL","operation":null}},{"value":{"original":"xterm-256color","status":""},"var":{"created":"2024-03-10T20:48:58.072104-04:00","key":"TERM","operation":null}},{"value":{"original":"fi=00:mi=00:mh=00:ln=01;36:or=01;31:di=01;34:ow=01;34:st=34:tw=34:pi=01;33:so=01;33:do=01;33:bd=01;33:cd=01;33:su=01;35:sg=01;35:ca=01;35:ex=01;32","status":""},"var":{"created":"2024-03-10T20:48:58.072106-04:00","key":"TREE_COLORS","operation":null}},{"value":{"original":"/Users/sourishkrout/Projects/stateful/2022Q4/wasi-sdk/dist/wasi-sdk-16.5ga0a342ac182c","status":""},"var":{"created":"2024-03-10T20:48:58.072111-04:00","key":"WASI_SDK_PATH","operation":null}},{"value":{"original":"/Users/sourishkrout/.begin","status":""},"var":{"created":"2024-03-10T20:48:58.072079-04:00","key":"BEGIN_INSTALL","operation":null}},{"value":{"original":"/opt/homebrew/Cellar","status":""},"var":{"created":"2024-03-10T20:48:58.072082-04:00","key":"HOMEBREW_CELLAR","operation":null}},{"value":{"original":"/opt/homebrew/share/google-cloud-sdk/bin:/Users/sourishkrout/.wasmtime/bin:/opt/homebrew/opt/libpq/bin:/Users/sourishkrout/go/bin:/Users/sourishkrout/.asdf/shims:/opt/homebrew/opt/asdf/libexec/bin:/Users/sourishkrout/bin:/opt/homebrew/bin:/opt/homebrew/sbin:/usr/local/bin:/System/Cryptexes/App/usr/bin:/var/run/com.apple.security.cryptexd/codex.system/bootstrap/usr/local/bin:/var/run/com.apple.security.cryptexd/codex.system/bootstrap/usr/bin:/var/run/com.apple.security.cryptexd/codex.system/bootstrap/usr/appleinternal/bin:/Library/Apple/usr/bin:/usr/bin:/bin:/usr/sbin:/sbin:/Users/sourishkrout/.cache/zsh4humans/v5/fzf/bin:/Applications/Postgres.app/Contents/Versions/16/bin","status":""},"var":{"created":"2024-03-10T20:48:58.072095-04:00","key":"PATH","operation":null}},{"value":{"original":"vs/workbench/api/node/extensionHostProcess","status":""},"var":{"created":"2024-03-10T20:48:58.072108-04:00","key":"VSCODE_AMD_ENTRYPOINT","operation":null}},{"value":{"original":"/opt/homebrew","status":""},"var":{"created":"2024-03-10T20:48:58.072083-04:00","key":"HOMEBREW_REPOSITORY","operation":null}},{"value":{"original":"en_US.UTF-8","status":""},"var":{"created":"2024-03-10T20:48:58.072087-04:00","key":"LC_ALL","operation":null}}],"load_1":[{"spec":{"checked":false,"description":"Some name","name":"Plain","required":true},"var":{"created":"2024-03-10T20:48:58.072167-04:00","key":"NAME","operation":null}},{"spec":{"checked":false,"description":"No idea what mode this is","name":"Plain","required":true},"var":{"created":"2024-03-10T20:48:58.072165-04:00","key":"COMMAND_MODE","operation":null}},{"spec":{"checked":false,"description":"User","name":"Plain","required":false},"var":{"created":"2024-03-10T20:48:58.072166-04:00","key":"USER","operation":null}},{"spec":{"checked":false,"description":"The message","name":"Plain","required":true},"var":{"created":"2024-03-10T20:48:58.072168-04:00","key":"MSG","operation":null}},{"spec":{"checked":false,"description":"Working directory","name":"Plain","required":false},"var":{"created":"2024-03-10T20:48:58.072167-04:00","key":"PWD","operation":null}},{"spec":{"checked":false,"description":"Some value","name":"Plain","required":false},"var":{"created":"2024-03-10T20:48:58.072167-04:00","key":"NAKED","operation":null}}],"reconcile_12":[{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:49:29.262762-04:00","key":"__","operation":null}}],"reconcile_3":[{"value":{"status":"UNRESOLVED"},"var":{"created":"2024-03-10T20:48:58.072317-04:00","key":"NAME","operation":null}},{"value":{"status":"UNRESOLVED"},"var":{"created":"2024-03-10T20:48:58.072318-04:00","key":"NAKED","operation":null}},{"value":{"status":"UNRESOLVED"},"var":{"created":"2024-03-10T20:48:58.072318-04:00","key":"MSG","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072307-04:00","key":"HOMEBREW_PREFIX","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072315-04:00","key":"TERM","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072316-04:00","key":"TREE_COLORS","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072316-04:00","key":"WASI_SDK_PATH","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072301-04:00","key":"TMPDIR","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072303-04:00","key":"LS_COLORS","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072304-04:00","key":"USE_GKE_GCLOUD_AUTH_PLUGIN","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072303-04:00","key":"KRAFTCLOUD_USER","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072308-04:00","key":"LOGNAME","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072312-04:00","key":"VSCODE_PID","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072313-04:00","key":"ELECTRON_RUN_AS_NODE","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.0723-04:00","key":"PATH","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072301-04:00","key":"SHLVL","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072301-04:00","key":"WASMTIME_HOME","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072307-04:00","key":"__CF_USER_TEXT_ENCODING","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072316-04:00","key":"HOMEBREW_CELLAR","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072298-04:00","key":"VSCODE_AMD_ENTRYPOINT","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072299-04:00","key":"HOMEBREW_REPOSITORY","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072306-04:00","key":"SSH_AUTH_SOCK","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.07231-04:00","key":"VSCODE_CRASH_REPORTER_PROCESS_TYPE","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072312-04:00","key":"_","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072313-04:00","key":"LSCOLORS","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072299-04:00","key":"LC_ALL","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072302-04:00","key":"GOPATH","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072308-04:00","key":"ASDF_DIR","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072316-04:00","key":"BEGIN_INSTALL","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072306-04:00","key":"VSCODE_NLS_CONFIG","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072307-04:00","key":"VSCODE_IPC_HOOK","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072309-04:00","key":"HOME","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072311-04:00","key":"VSCODE_HANDLES_UNCAUGHT_ERRORS","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072312-04:00","key":"LESS","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072315-04:00","key":"MANPATH","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072302-04:00","key":"APPLICATION_INSIGHTS_NO_DIAGNOSTIC_CHANNEL","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072304-04:00","key":"VSCODE_CWD","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072305-04:00","key":"PAGER","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072314-04:00","key":"XPC_SERVICE_NAME","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072315-04:00","key":"SHELL","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072306-04:00","key":"__CFBundleIdentifier","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072308-04:00","key":"XPC_FLAGS","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072314-04:00","key":"TERMINFO","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072317-04:00","key":"ORIGINAL_XDG_CURRENT_DESKTOP","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.0723-04:00","key":"MallocNanoZone","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072309-04:00","key":"INFOPATH","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072313-04:00","key":"OLDPWD","operation":null}}],"reconcile_6":[{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:49:29.040802-04:00","key":"RUNME_ID","operation":null}}],"reconcile_9":[{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:49:29.228105-04:00","key":"INNER","operation":null}}],"update_10":[{"value":{"original":"Hello world, LPT!\r\nsourishkrout\r\n","status":""},"var":{"created":"2024-03-10T20:49:29.262703-04:00","key":"__","operation":null}}],"update_13":[{"value":{"original":"01HRF7RKVNFZCZ9P8GH7CNHZSY","status":""},"var":{"created":"2024-03-10T20:49:36.136807-04:00","key":"RUNME_ID","operation":null}},{"value":{"original":"xterm-256color","status":""},"var":{"created":"2024-03-10T20:49:36.136809-04:00","key":"TERM","operation":null}}],"update_17":[{"value":{"original":"Hello world\r\n123\r\nsourishkrout\r\nLPT\r\n/Users/sourishkrout/Projects/stateful/oss/vscode-runme/examples\r\nnested\r\n","status":""},"var":{"created":"2024-03-10T20:49:36.414365-04:00","key":"__","operation":null}}],"update_4":[{"value":{"original":"01HQ64XAYM289P5DF7CS0EJ54N","status":""},"var":{"created":"2024-03-10T20:49:29.040753-04:00","key":"RUNME_ID","operation":null}},{"value":{"original":"xterm-256color","status":""},"var":{"created":"2024-03-10T20:49:29.040755-04:00","key":"TERM","operation":null}}],"update_7":[{"value":{"original":"Hello world","status":""},"var":{"created":"2024-03-10T20:49:29.228017-04:00","key":"MSG","operation":null}},{"value":{"original":"nested","status":""},"var":{"created":"2024-03-10T20:49:29.228019-04:00","key":"INNER","operation":null}},{"value":{"original":"LPT","status":""},"var":{"created":"2024-03-10T20:49:29.228019-04:00","key":"NAME","operation":null}},{"value":{"original":"123","status":""},"var":{"created":"2024-03-10T20:49:29.22802-04:00","key":"NAKED","operation":null}}]}`), &vars)
	require.NoError(t, err)

	fakeQuery := `query ResolveOwlDotEnv($insecure: Boolean = false, $prefix: String = "", $load_0: [VariableInput]!, $load_1: [VariableInput]!, $reconcile_3: [VariableInput]!, $update_4: [VariableInput]!, $reconcile_6: [VariableInput]!, $update_7: [VariableInput]!, $reconcile_9: [VariableInput]!, $update_10: [VariableInput]!, $reconcile_12: [VariableInput]!, $update_13: [VariableInput]!, $update_17: [VariableInput]!) {
  environment {
    load(vars: $load_0, hasSpecs: false) {
      load(vars: $load_1, hasSpecs: true) {
        reconcile(vars: $reconcile_3, hasSpecs: true) {
          update(vars: $update_4, hasSpecs: false) {
            reconcile(vars: $reconcile_6, hasSpecs: true) {
              update(vars: $update_7, hasSpecs: false) {
                reconcile(vars: $reconcile_9, hasSpecs: true) {
                  update(vars: $update_10, hasSpecs: false) {
                    reconcile(vars: $reconcile_12, hasSpecs: true) {
                      update(vars: $update_13, hasSpecs: false) {
                        update(vars: $update_17, hasSpecs: false) {
                          validate {
                            Opaque(insecure: $insecure, keys: ["ELECTRON_RUN_AS_NODE", "PAGER", "INFOPATH", "VSCODE_PID", "__CF_USER_TEXT_ENCODING", "BEGIN_INSTALL", "TMPDIR", "HOMEBREW_PREFIX", "HOMEBREW_REPOSITORY", "LC_ALL", "VSCODE_CRASH_REPORTER_PROCESS_TYPE", "VSCODE_NLS_CONFIG", "USE_GKE_GCLOUD_AUTH_PLUGIN", "ASDF_DIR", "LOGNAME", "VSCODE_AMD_ENTRYPOINT", "HOME", "VSCODE_HANDLES_UNCAUGHT_ERRORS", "__CFBundleIdentifier", "WASI_SDK_PATH", "TERMINFO", "KRAFTCLOUD_USER", "INSTRUMENTATION_KEY", "APPLICATION_INSIGHTS_NO_DIAGNOSTIC_CHANNEL", "TERM", "GOPATH", "_", "MANPATH", "SHLVL", "HOMEBREW_CELLAR", "TREE_COLORS", "RUNME_ID", "INNER", "MallocNanoZone", "OLDPWD", "VSCODE_IPC_HOOK", "XPC_SERVICE_NAME", "SHELL", "ORIGINAL_XDG_CURRENT_DESKTOP", "XPC_FLAGS", "WASMTIME_HOME", "SSH_AUTH_SOCK", "LS_COLORS", "PATH", "BUF_TOKEN", "LESS", "VSCODE_CWD", "LSCOLORS", "__"]) {
                              name
                              sensitive
                              mask
                              Plain(insecure: $insecure, keys: ["MSG", "PWD", "NAKED", "NAME", "USER", "COMMAND_MODE", "OPENAI_ORG_ID"]) {
                                name
                                sensitive
                                mask
                                Secret(insecure: $insecure, keys: ["OPENAI_API_KEY", "KRAFTCLOUD_TOKEN"]) {
                                  name
                                  sensitive
                                  mask
                                  done {
                                    render {
                                      dotenv(insecure: $insecure, prefix: $prefix)
                                    }
                                  }
                                }
                              }
                            }
                          }
                        }
                      }
                    }
                  }
                }
              }
            }
          }
        }
      }
    }
  }
}`

	t.Run("without prefix", func(t *testing.T) {
		result := graphql.Do(graphql.Params{
			Schema:         Schema,
			RequestString:  fakeQuery,
			VariableValues: vars,
		})

		require.False(t, result.HasErrors())

		dotenv, err := extractDataKey(result.Data, "dotenv")
		require.NoError(t, err)
		require.NotNil(t, dotenv)

		dotenvStr, ok := dotenv.(string)
		require.True(t, ok)
		require.False(t, strings.HasPrefix(dotenvStr, "VITE_APP"))

		fmt.Println(dotenvStr)
	})

	t.Run("with VITE_APP prefix", func(t *testing.T) {
		prefix := "VITE_APP_"
		vars["prefix"] = prefix
		vars["insecure"] = false
		result := graphql.Do(graphql.Params{
			Schema:         Schema,
			RequestString:  fakeQuery,
			VariableValues: vars,
		})

		require.False(t, result.HasErrors())

		dotenv, err := extractDataKey(result.Data, "dotenv")
		require.NoError(t, err)
		require.NotNil(t, dotenv)

		dotenvStr, ok := dotenv.(string)
		require.True(t, ok)
		require.True(t, strings.HasPrefix(dotenvStr, prefix))

		nlines := len(strings.Split(dotenvStr, "\n"))
		require.EqualValues(t, 7, nlines)
	})

	t.Run("with insecure", func(t *testing.T) {
		vars["insecure"] = true
		result := graphql.Do(graphql.Params{
			Schema:         Schema,
			RequestString:  fakeQuery,
			VariableValues: vars,
		})

		require.False(t, result.HasErrors())

		dotenv, err := extractDataKey(result.Data, "dotenv")
		require.NoError(t, err)
		require.NotNil(t, dotenv)

		dotenvStr, ok := dotenv.(string)
		require.True(t, ok)
		nlines := len(strings.Split(dotenvStr, "\n"))
		require.EqualValues(t, 60, nlines)
	})
}

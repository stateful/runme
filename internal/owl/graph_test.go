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
		err := json.Unmarshal([]byte(`{"load_0":[{"var":{"key":"GOPATH","created":"2024-03-02T13:25:01.270468-05:00","operation":null},"spec":{"checked":false,"name":"Opaque","required":false},"value":{"resolved":"/Users/sourishkrout/go","status":""}},{"var":{"key":"HOME","created":"2024-03-02T13:25:01.270469-05:00","operation":null},"spec":{"checked":false,"name":"Opaque","required":false},"value":{"resolved":"/Users/sourishkrout","status":""}},{"var":{"key":"HOMEBREW_REPOSITORY","created":"2024-03-02T13:25:01.270471-05:00","operation":null},"spec":{"checked":false,"name":"Opaque","required":false},"value":{"resolved":"/opt/homebrew","status":""}}]}`), &vars)
		require.NoError(t, err)

		result := graphql.Do(graphql.Params{
			Schema: Schema,
			RequestString: `query ResolveOwlSnapshot($insecure: Boolean = false, $load_0: [VariableInput]!) {
  environment {
    load(vars: $load_0, hasSpecs: false) {
      location
      snapshot(insecure: $insecure) {
        var {
          key
          created
          updated
        }
        value {
          original
          resolved
          status
        }
        spec {
          name
          required
        }
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

	// t.Run("query complex environment", func(t *testing.T) {
	// 	jsonVars := `{"insecure":false,"load_0":[{"created":"2024-03-08T13:58:59.798507-05:00","key":"TREE_COLORS","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"fi=00:mi=00:mh=00:ln=01;36:or=01;31:di=01;34:ow=01;34:st=34:tw=34:pi=01;33:so=01;33:do=01;33:bd=01;33:cd=01;33:su=01;35:sg=01;35:ca=01;35:ex=01;32","status":""}},{"created":"2024-03-08T13:58:59.79851-05:00","key":"VSCODE_NLS_CONFIG","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"{\"locale\":\"en-us\",\"osLocale\":\"en-us\",\"availableLanguages\":{},\"_languagePackSupport\":true}","status":""}},{"created":"2024-03-08T13:58:59.798511-05:00","key":"XPC_FLAGS","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"0x0","status":""}},{"created":"2024-03-08T13:58:59.798514-05:00","key":"__CF_USER_TEXT_ENCODING","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"0x1F5:0x0:0x0","status":""}},{"created":"2024-03-08T13:58:59.798516-05:00","key":"APPLICATION_INSIGHTS_NO_DIAGNOSTIC_CHANNEL","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"1","status":""}},{"created":"2024-03-08T13:58:59.798505-05:00","key":"TERMINFO","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"/Users/sourishkrout/.terminfo","status":""}},{"created":"2024-03-08T13:58:59.79849-05:00","key":"COMMAND_MODE","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"unix2003","status":""}},{"created":"2024-03-08T13:58:59.798493-05:00","key":"INFOPATH","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"/opt/homebrew/share/info:","status":""}},{"created":"2024-03-08T13:58:59.798511-05:00","key":"WASMTIME_HOME","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"/Users/sourishkrout/.wasmtime","status":""}},{"created":"2024-03-08T13:58:59.798487-05:00","key":"ASDF_DIR","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"/opt/homebrew/opt/asdf/libexec","status":""}},{"created":"2024-03-08T13:58:59.798489-05:00","key":"BUF_TOKEN","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"d8ccb8599cABCDEF1235324235ZDFHRAKSDFJAEW123kasdfasdf1231230fe188","status":""}},{"created":"2024-03-08T13:58:59.798512-05:00","key":"XPC_SERVICE_NAME","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"application.com.microsoft.VSCode.251091548.251091554","status":""}},{"created":"2024-03-08T13:58:59.79849-05:00","key":"GOPATH","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"/Users/sourishkrout/go","status":""}},{"created":"2024-03-08T13:58:59.798491-05:00","key":"HOMEBREW_PREFIX","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"/opt/homebrew","status":""}},{"created":"2024-03-08T13:58:59.798508-05:00","key":"USER","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"sourishkrout","status":""}},{"created":"2024-03-08T13:58:59.79851-05:00","key":"VSCODE_IPC_HOOK","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"/Users/sourishkrout/Library/Application Support/Code/1.87-main.sock","status":""}},{"created":"2024-03-08T13:58:59.798492-05:00","key":"HOMEBREW_REPOSITORY","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"/opt/homebrew","status":""}},{"created":"2024-03-08T13:58:59.798499-05:00","key":"LS_COLORS","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"fi=00:mi=00:mh=00:ln=01;36:or=01;31:di=01;34:ow=04;01;34:st=34:tw=04;34:pi=01;33:so=01;33:do=01;33:bd=01;33:cd=01;33:su=01;35:sg=01;35:ca=01;35:ex=01;32","status":""}},{"created":"2024-03-08T13:58:59.798504-05:00","key":"SHELL","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"/bin/zsh","status":""}},{"created":"2024-03-08T13:58:59.798504-05:00","key":"SHLVL","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"0","status":""}},{"created":"2024-03-08T13:58:59.798505-05:00","key":"TERM","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"xterm-256color","status":""}},{"created":"2024-03-08T13:58:59.798514-05:00","key":"ELECTRON_RUN_AS_NODE","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"1","status":""}},{"created":"2024-03-08T13:58:59.798494-05:00","key":"INSTRUMENTATION_KEY","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"xxxxxxxx-5101-xxxx-a0d0-xxxxxxxxxxxx","status":""}},{"created":"2024-03-08T13:58:59.798502-05:00","key":"PAGER","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"less","status":""}},{"created":"2024-03-08T13:58:59.79851-05:00","key":"VSCODE_PID","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"89716","status":""}},{"created":"2024-03-08T13:58:59.798516-05:00","key":"NAME","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"Luna","status":""}},{"created":"2024-03-08T13:58:59.798491-05:00","key":"HOME","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"/Users/sourishkrout","status":""}},{"created":"2024-03-08T13:58:59.798501-05:00","key":"OPENAI_ORG_ID","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"org-tm5BAbynhBsE9Lzy1HTD6sk0","status":""}},{"created":"2024-03-08T13:58:59.798506-05:00","key":"TMPDIR","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"/var/folders/c3/5r0t1nzs7sbfpxjgbc6n3ss40000gn/T/","status":""}},{"created":"2024-03-08T13:58:59.798496-05:00","key":"LOGNAME","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"sourishkrout","status":""}},{"created":"2024-03-08T13:58:59.7985-05:00","key":"MANPATH","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"/opt/homebrew/share/man:/usr/share/man:/usr/local/share/man:/Users/sourishkrout/.cache/zsh4humans/v5/fzf/man:","status":""}},{"created":"2024-03-08T13:58:59.7985-05:00","key":"MallocNanoZone","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"0","status":""}},{"created":"2024-03-08T13:58:59.798502-05:00","key":"PATH","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"/opt/homebrew/share/google-cloud-sdk/bin:/Users/sourishkrout/.wasmtime/bin:/opt/homebrew/opt/libpq/bin:/Users/sourishkrout/go/bin:/Users/sourishkrout/.asdf/shims:/opt/homebrew/opt/asdf/libexec/bin:/Users/sourishkrout/bin:/opt/homebrew/bin:/opt/homebrew/sbin:/usr/local/bin:/System/Cryptexes/App/usr/bin:/var/run/com.apple.security.cryptexd/codex.system/bootstrap/usr/local/bin:/var/run/com.apple.security.cryptexd/codex.system/bootstrap/usr/bin:/var/run/com.apple.security.cryptexd/codex.system/bootstrap/usr/appleinternal/bin:/Library/Apple/usr/bin:/usr/bin:/bin:/usr/sbin:/sbin:/Users/sourishkrout/.cache/zsh4humans/v5/fzf/bin:/Applications/Postgres.app/Contents/Versions/16/bin","status":""}},{"created":"2024-03-08T13:58:59.798509-05:00","key":"VSCODE_HANDLES_UNCAUGHT_ERRORS","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"true","status":""}},{"created":"2024-03-08T13:58:59.798513-05:00","key":"__CFBundleIdentifier","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"com.microsoft.VSCode","status":""}},{"created":"2024-03-08T13:58:59.798491-05:00","key":"HOMEBREW_CELLAR","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"/opt/homebrew/Cellar","status":""}},{"created":"2024-03-08T13:58:59.798496-05:00","key":"LESS","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"-iRFXMx4","status":""}},{"created":"2024-03-08T13:58:59.7985-05:00","key":"OLDPWD","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"/","status":""}},{"created":"2024-03-08T13:58:59.798501-05:00","key":"OPENAI_API_KEY","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"sk-KhRA3yB6adghat3tgiasb8v4ahsdifhasGaS33ZNg4gTdtBq","status":""}},{"created":"2024-03-08T13:58:59.798508-05:00","key":"USE_GKE_GCLOUD_AUTH_PLUGIN","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"True","status":""}},{"created":"2024-03-08T13:58:59.798495-05:00","key":"KRAFTCLOUD_USER","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"achristian","status":""}},{"created":"2024-03-08T13:58:59.798497-05:00","key":"LSCOLORS","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"ExGxDxDxCxDxDxFxFxexEx","status":""}},{"created":"2024-03-08T13:58:59.798494-05:00","key":"KRAFTCLOUD_TOKEN","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"ca3adk4iajfd84xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxvWEZXZDlQZjRNZUkyblFl","status":""}},{"created":"2024-03-08T13:58:59.798505-05:00","key":"SSH_AUTH_SOCK","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"/private/tmp/com.apple.launchd.WJncT7ZrHW/Listeners","status":""}},{"created":"2024-03-08T13:58:59.798508-05:00","key":"VSCODE_AMD_ENTRYPOINT","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"vs/workbench/api/node/extensionHostProcess","status":""}},{"created":"2024-03-08T13:58:59.798511-05:00","key":"WASI_SDK_PATH","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"/Users/sourishkrout/Projects/stateful/2022Q4/wasi-sdk/dist/wasi-sdk-16.5ga0a342ac182c","status":""}},{"created":"2024-03-08T13:58:59.798512-05:00","key":"_","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"/Applications/Visual Studio Code.app/Contents/MacOS/Electron","status":""}},{"created":"2024-03-08T13:58:59.798509-05:00","key":"VSCODE_CRASH_REPORTER_PROCESS_TYPE","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"extensionHost","status":""}},{"created":"2024-03-08T13:58:59.798509-05:00","key":"VSCODE_CWD","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"/","status":""}},{"created":"2024-03-08T13:58:59.798501-05:00","key":"ORIGINAL_XDG_CURRENT_DESKTOP","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"undefined","status":""}},{"created":"2024-03-08T13:58:59.798489-05:00","key":"BEGIN_INSTALL","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"/Users/sourishkrout/.begin","status":""}},{"created":"2024-03-08T13:58:59.798495-05:00","key":"LC_ALL","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"en_US.UTF-8","status":""}},{"created":"2024-03-08T13:58:59.798503-05:00","key":"PWD","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"/","status":""}}],"load_1":[{"created":"2024-03-08T13:58:59.798555-05:00","key":"USER","operation":null,"spec":{"checked":false,"name":"Plain","required":false},"value":{"original":"User","status":""}},{"created":"2024-03-08T13:58:59.798556-05:00","key":"OPENAI_API_KEY","operation":null,"spec":{"checked":false,"name":"Secret","required":false},"value":{"original":"Your OpenAI API key matching the org","status":""}},{"created":"2024-03-08T13:58:59.798557-05:00","key":"KRAFTCLOUD_TOKEN","operation":null,"spec":{"checked":false,"name":"Secret","required":false},"value":{"original":"This is secret","status":""}},{"created":"2024-03-08T13:58:59.798558-05:00","key":"MSG","operation":null,"spec":{"checked":false,"name":"Plain","required":false},"value":{"original":"The message","status":""}},{"created":"2024-03-08T13:58:59.798556-05:00","key":"NAME","operation":null,"spec":{"checked":false,"name":"Plain","required":false},"value":{"original":"This is a name","status":""}},{"created":"2024-03-08T13:58:59.798557-05:00","key":"COMMAND_MODE","operation":null,"spec":{"checked":false,"name":"Plain","required":false},"value":{"original":"No idea what mode this is","status":""}},{"created":"2024-03-08T13:58:59.798557-05:00","key":"NAKED","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"# Plain","status":""}},{"created":"2024-03-08T13:58:59.798558-05:00","key":"PWD","operation":null,"spec":{"checked":false,"name":"Plain","required":false},"value":{"original":"Working directory","status":""}},{"created":"2024-03-08T13:58:59.798558-05:00","key":"OPENAI_ORG_ID","operation":null,"spec":{"checked":false,"name":"Plain","required":false},"value":{"original":"Your OpenAI org identifier","status":""}}],"load_2":[{"created":"2024-03-08T13:58:59.798561-05:00","key":"NAME","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"Luna","status":""}}],"update_3":[{"created":"2024-03-08T14:00:16.057889-05:00","key":"RUNME_ID","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"01HQ64XAYM289P5DF7CS0EJ54N","status":""}},{"created":"2024-03-08T14:00:16.05789-05:00","key":"NAME","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"Luna","status":""}},{"created":"2024-03-08T14:00:16.057891-05:00","key":"TERM","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"xterm-256color","status":""}}],"update_5":[{"created":"2024-03-08T14:00:16.261241-05:00","key":"INNER","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"nested","status":""}}],"update_7":[{"created":"2024-03-08T14:00:16.288126-05:00","key":"__","operation":null,"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"The message, Luna!\r\nsourishkrout\r\n","status":""}}]}`
	// 	query := `query ResolveOwlSnapshot($insecure: Boolean = false, $load_0: [VariableInput]!, $load_1: [VariableInput]!, $load_2: [VariableInput]!, $update_3: [VariableInput]!, $update_5: [VariableInput]!, $update_7: [VariableInput]!) {
	//   environment {
	//     load(vars: $load_0, location: "[system]", hasSpecs: false) {
	//       load(vars: $load_1, location: ".env.example", hasSpecs: true) {
	//         load(vars: $load_2, location: ".env", hasSpecs: false) {
	//           update(vars: $update_3, location: "exec", hasSpecs: false) {
	//             update(vars: $update_5, location: "exec", hasSpecs: false) {
	//               update(vars: $update_7, location: "exec", hasSpecs: false) {
	//                 validate {
	//                   Opaque(insecure: $insecure, keys: ["VSCODE_NLS_CONFIG", "LS_COLORS", "INSTRUMENTATION_KEY", "MANPATH", "VSCODE_HANDLES_UNCAUGHT_ERRORS", "RUNME_ID", "VSCODE_CWD", "APPLICATION_INSIGHTS_NO_DIAGNOSTIC_CHANNEL", "TERM", "HOMEBREW_REPOSITORY", "HOME", "LC_ALL", "BEGIN_INSTALL", "USE_GKE_GCLOUD_AUTH_PLUGIN", "VSCODE_CRASH_REPORTER_PROCESS_TYPE", "__CF_USER_TEXT_ENCODING", "WASMTIME_HOME", "BUF_TOKEN", "TMPDIR", "LESS", "ORIGINAL_XDG_CURRENT_DESKTOP", "HOMEBREW_CELLAR", "SSH_AUTH_SOCK", "__", "WASI_SDK_PATH", "SHLVL", "ELECTRON_RUN_AS_NODE", "NAME", "MallocNanoZone", "XPC_FLAGS", "XPC_SERVICE_NAME", "INNER", "VSCODE_IPC_HOOK", "PATH", "ASDF_DIR", "__CFBundleIdentifier", "OLDPWD", "TREE_COLORS", "LSCOLORS", "SHELL", "VSCODE_PID", "GOPATH", "PAGER", "LOGNAME", "_", "HOMEBREW_PREFIX", "TERMINFO", "KRAFTCLOUD_USER", "VSCODE_AMD_ENTRYPOINT", "NAKED", "INFOPATH"]) {
	//                     spec
	//                     sensitive
	//                     mask
	//                     errors
	//                     Plain(insecure: $insecure, keys: ["PWD", "COMMAND_MODE", "OPENAI_ORG_ID", "USER", "MSG"]) {
	//                       spec
	//                       sensitive
	//                       mask
	//                       errors
	//                       Secret(insecure: $insecure, keys: ["KRAFTCLOUD_TOKEN", "OPENAI_API_KEY"]) {
	//                         spec
	//                         sensitive
	//                         mask
	//                         errors
	//                         done {
	//                           snapshot(insecure: $insecure) {
	//                             var {
	//                               key
	//                               created
	//                               updated
	//                             }
	//                             operation {
	//                               location
	//                             }
	//                             value {
	//                               original
	//                               resolved
	//                               status
	//                             }
	//                             spec {
	//                               name
	//                               required
	//                             }
	//                           }
	//                         }
	//                       }
	//                     }
	//                   }
	//                 }
	//               }
	//             }
	//           }
	//         }
	//       }
	//     }
	//   }
	// }`
	// 	var vars map[string]interface{}
	// 	err := json.Unmarshal([]byte(jsonVars), &vars)
	// 	require.NoError(t, err)

	// 	result := graphql.Do(graphql.Params{
	// 		Schema:         Schema,
	// 		RequestString:  query,
	// 		VariableValues: vars,
	// 	})

	// 	fmt.Println(result.Errors)
	// 	require.False(t, result.HasErrors())

	// 	b, _ := json.MarshalIndent(result, "", " ")
	// 	fmt.Println(string(b))
	// })

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
		err := json.Unmarshal([]byte(`{"load_0":[{"var":{"key":"GOPATH","created":"2024-03-02T13:25:01.270468-05:00","operation":null},"spec":{"checked":false,"name":"Opaque","required":false},"value":{"resolved":"/Users/sourishkrout/go","status":""}},{"var":{"created":"2024-03-02T13:25:01.270469-05:00","key":"HOME","operation":null},"spec":{"checked":false,"name":"Secret","required":true},"value":{"resolved":"/Users/sourishkrout","status":""}},{"var":{"created":"2024-03-02T13:25:01.270471-05:00","key":"HOMEBREW_REPOSITORY","operation":null},"spec":{"checked":false,"name":"Plain","required":false},"value":{"resolved":"/opt/homebrew","status":""}}]}`), &vars)
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
                  var {
                    key
                    created
                    updated
                  }
                  value {
                    original
                    resolved
                    status
                  }
                  spec {
                    name
                    required
                  }
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

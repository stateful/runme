package owl

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/testutil"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
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

	t.Run("query simple environment", func(t *testing.T) {
		var vars map[string]interface{}
		err := json.Unmarshal([]byte(`{"load_0":[{"var":{"key":"GOPATH","created":"2024-03-02T13:25:01.270468-05:00","operation":null},"spec":{"checked":false,"name":"Opaque","required":false},"value":{"resolved":"/Users/sourishkrout/go","status":""}},{"var":{"key":"HOME","created":"2024-03-02T13:25:01.270469-05:00","operation":null},"spec":{"checked":false,"name":"Opaque","required":false},"value":{"resolved":"/Users/sourishkrout","status":""}},{"var":{"key":"HOMEBREW_REPOSITORY","created":"2024-03-02T13:25:01.270471-05:00","operation":null},"spec":{"checked":false,"name":"Opaque","required":false},"value":{"resolved":"/opt/homebrew","status":""}}]}`), &vars)
		require.NoError(t, err)

		result := graphql.Do(graphql.Params{
			Schema: Schema,
			RequestString: `query ResolveOwlSnapshot($insecure: Boolean = false, $load_0: [VariableInput]!) {
  environment {
    load(vars: $load_0, hasSpecs: false) {
      render {
        snapshot(insecure: $insecure) {
          var {
            key
            origin
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
          errors {
            code
            message
          }
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

		snapshot := result.Data.(map[string]interface{})["environment"].(map[string]interface{})["load"].(map[string]interface{})["render"].(map[string]interface{})["snapshot"]
		require.NoError(t, err)
		require.Len(t, snapshot, 3)
	})

	t.Run("query complex environment", func(t *testing.T) {
		jsonVars := `{"insecure":true,"load_0":[{"value":{"original":"/opt/homebrew/share/man:/usr/share/man:/usr/local/share/man:/Users/sourishkrout/.cache/zsh4humans/v5/fzf/man:","status":""},"var":{"created":"2024-03-11T16:42:52.999456-04:00","key":"MANPATH","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/private/tmp/com.apple.launchd.WJncT7ZrHW/Listeners","status":""},"var":{"created":"2024-03-11T16:42:52.999462-04:00","key":"SSH_AUTH_SOCK","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"89716","status":""},"var":{"created":"2024-03-11T16:42:52.999469-04:00","key":"VSCODE_PID","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/opt/homebrew","status":""},"var":{"created":"2024-03-11T16:42:52.999447-04:00","key":"HOMEBREW_PREFIX","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"en_US.UTF-8","status":""},"var":{"created":"2024-03-11T16:42:52.999452-04:00","key":"LC_ALL","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"-iRFXMx4","status":""},"var":{"created":"2024-03-11T16:42:52.999453-04:00","key":"LESS","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx","status":""},"var":{"created":"2024-03-11T16:42:52.999457-04:00","key":"OPENAI_API_KEY","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/","status":""},"var":{"created":"2024-03-11T16:42:52.999459-04:00","key":"PWD","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"{\"locale\":\"en-us\",\"osLocale\":\"en-us\",\"availableLanguages\":{},\"_languagePackSupport\":true}","status":""},"var":{"created":"2024-03-11T16:42:52.999468-04:00","key":"VSCODE_NLS_CONFIG","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/opt/homebrew","status":""},"var":{"created":"2024-03-11T16:42:52.999448-04:00","key":"HOMEBREW_REPOSITORY","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"cmxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxlFl","status":""},"var":{"created":"2024-03-11T16:42:52.999451-04:00","key":"KRAFTCLOUD_TOKEN","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"ExGxDxDxCxDxDxFxFxexEx","status":""},"var":{"created":"2024-03-11T16:42:52.999454-04:00","key":"LSCOLORS","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"0","status":""},"var":{"created":"2024-03-11T16:42:52.999456-04:00","key":"MallocNanoZone","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/bin/zsh","status":""},"var":{"created":"2024-03-11T16:42:52.99946-04:00","key":"SHELL","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"xterm-256color","status":""},"var":{"created":"2024-03-11T16:42:52.999463-04:00","key":"TERM","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/Applications/Visual Studio Code.app/Contents/MacOS/Electron","status":""},"var":{"created":"2024-03-11T16:42:52.999471-04:00","key":"_","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"1","status":""},"var":{"created":"2024-03-11T16:42:52.999472-04:00","key":"APPLICATION_INSIGHTS_NO_DIAGNOSTIC_CHANNEL","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/Users/sourishkrout/.begin","status":""},"var":{"created":"2024-03-11T16:42:52.999444-04:00","key":"BEGIN_INSTALL","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"less","status":""},"var":{"created":"2024-03-11T16:42:52.999458-04:00","key":"PAGER","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/Users/sourishkrout/.terminfo","status":""},"var":{"created":"2024-03-11T16:42:52.999463-04:00","key":"TERMINFO","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/","status":""},"var":{"created":"2024-03-11T16:42:52.999467-04:00","key":"VSCODE_CWD","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/Users/sourishkrout/Library/Application Support/Code/1.87-main.sock","status":""},"var":{"created":"2024-03-11T16:42:52.999468-04:00","key":"VSCODE_IPC_HOOK","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/Users/sourishkrout/Projects/stateful/2022Q4/wasi-sdk/dist/wasi-sdk-16.5ga0a342ac182c","status":""},"var":{"created":"2024-03-11T16:42:52.999469-04:00","key":"WASI_SDK_PATH","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"1","status":""},"var":{"created":"2024-03-11T16:42:52.999472-04:00","key":"ELECTRON_RUN_AS_NODE","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/opt/homebrew/opt/asdf/libexec","status":""},"var":{"created":"2024-03-11T16:42:52.999443-04:00","key":"ASDF_DIR","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/Users/sourishkrout","status":""},"var":{"created":"2024-03-11T16:42:52.999446-04:00","key":"HOME","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/opt/homebrew/share/google-cloud-sdk/bin:/Users/sourishkrout/.wasmtime/bin:/opt/homebrew/opt/libpq/bin:/Users/sourishkrout/go/bin:/Users/sourishkrout/.asdf/shims:/opt/homebrew/opt/asdf/libexec/bin:/Users/sourishkrout/bin:/opt/homebrew/bin:/opt/homebrew/sbin:/usr/local/bin:/System/Cryptexes/App/usr/bin:/var/run/com.apple.security.cryptexd/codex.system/bootstrap/usr/local/bin:/var/run/com.apple.security.cryptexd/codex.system/bootstrap/usr/bin:/var/run/com.apple.security.cryptexd/codex.system/bootstrap/usr/appleinternal/bin:/Library/Apple/usr/bin:/usr/bin:/bin:/usr/sbin:/sbin:/Users/sourishkrout/.cache/zsh4humans/v5/fzf/bin:/Applications/Postgres.app/Contents/Versions/16/bin","status":""},"var":{"created":"2024-03-11T16:42:52.999459-04:00","key":"PATH","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/var/folders/c3/5r0t1nzs7sbfpxjgbc6n3ss40000gn/T/","status":""},"var":{"created":"2024-03-11T16:42:52.999464-04:00","key":"TMPDIR","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"fi=00:mi=00:mh=00:ln=01;36:or=01;31:di=01;34:ow=01;34:st=34:tw=34:pi=01;33:so=01;33:do=01;33:bd=01;33:cd=01;33:su=01;35:sg=01;35:ca=01;35:ex=01;32","status":""},"var":{"created":"2024-03-11T16:42:52.999465-04:00","key":"TREE_COLORS","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"extensionHost","status":""},"var":{"created":"2024-03-11T16:42:52.999467-04:00","key":"VSCODE_CRASH_REPORTER_PROCESS_TYPE","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"true","status":""},"var":{"created":"2024-03-11T16:42:52.999467-04:00","key":"VSCODE_HANDLES_UNCAUGHT_ERRORS","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"application.com.microsoft.VSCode.251091548.251091554","status":""},"var":{"created":"2024-03-11T16:42:52.99947-04:00","key":"XPC_SERVICE_NAME","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"d8xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxe188","status":""},"var":{"created":"2024-03-11T16:42:52.999445-04:00","key":"BUF_TOKEN","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"unix2003","status":""},"var":{"created":"2024-03-11T16:42:52.999445-04:00","key":"COMMAND_MODE","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/Users/sourishkrout/go","status":""},"var":{"created":"2024-03-11T16:42:52.999446-04:00","key":"GOPATH","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/opt/homebrew/Cellar","status":""},"var":{"created":"2024-03-11T16:42:52.999447-04:00","key":"HOMEBREW_CELLAR","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"sourishkrout","status":""},"var":{"created":"2024-03-11T16:42:52.999454-04:00","key":"LOGNAME","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"org-tm5BAbynhBsExxxxxxxxxxxx","status":""},"var":{"created":"2024-03-11T16:42:52.999457-04:00","key":"OPENAI_ORG_ID","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"sourishkrout","status":""},"var":{"created":"2024-03-11T16:42:52.999465-04:00","key":"USER","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/Users/sourishkrout/.wasmtime","status":""},"var":{"created":"2024-03-11T16:42:52.999469-04:00","key":"WASMTIME_HOME","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/opt/homebrew/share/info:","status":""},"var":{"created":"2024-03-11T16:42:52.999451-04:00","key":"INFOPATH","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"achristian","status":""},"var":{"created":"2024-03-11T16:42:52.999452-04:00","key":"KRAFTCLOUD_USER","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"fi=00:mi=00:mh=00:ln=01;36:or=01;31:di=01;34:ow=04;01;34:st=34:tw=04;34:pi=01;33:so=01;33:do=01;33:bd=01;33:cd=01;33:su=01;35:sg=01;35:ca=01;35:ex=01;32","status":""},"var":{"created":"2024-03-11T16:42:52.999455-04:00","key":"LS_COLORS","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/","status":""},"var":{"created":"2024-03-11T16:42:52.999457-04:00","key":"OLDPWD","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"0","status":""},"var":{"created":"2024-03-11T16:42:52.999462-04:00","key":"SHLVL","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"True","status":""},"var":{"created":"2024-03-11T16:42:52.999466-04:00","key":"USE_GKE_GCLOUD_AUTH_PLUGIN","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"vs/workbench/api/node/extensionHostProcess","status":""},"var":{"created":"2024-03-11T16:42:52.999466-04:00","key":"VSCODE_AMD_ENTRYPOINT","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"com.microsoft.VSCode","status":""},"var":{"created":"2024-03-11T16:42:52.999471-04:00","key":"__CFBundleIdentifier","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"0x1F5:0x0:0x0","status":""},"var":{"created":"2024-03-11T16:42:52.999471-04:00","key":"__CF_USER_TEXT_ENCODING","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx","status":""},"var":{"created":"2024-03-11T16:42:52.999451-04:00","key":"INSTRUMENTATION_KEY","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"undefined","status":""},"var":{"created":"2024-03-11T16:42:52.999458-04:00","key":"ORIGINAL_XDG_CURRENT_DESKTOP","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"0x0","status":""},"var":{"created":"2024-03-11T16:42:52.99947-04:00","key":"XPC_FLAGS","operation":{"order":0,"source":"[system]"}}}],"load_1":[{"spec":{"checked":false,"description":"Your OpenAI API key matching the org","name":"Secret","required":true},"var":{"created":"2024-03-11T16:42:52.999505-04:00","key":"OPENAI_API_KEY","operation":{"order":0,"source":".env.example"}}},{"spec":{"checked":false,"description":"Your OpenAI org identifier","name":"Plain","required":true},"var":{"created":"2024-03-11T16:42:52.999506-04:00","key":"OPENAI_ORG_ID","operation":{"order":0,"source":".env.example"}}},{"spec":{"checked":false,"description":"This is secret","name":"Password","required":true},"var":{"created":"2024-03-11T16:42:52.999506-04:00","key":"KRAFTCLOUD_TOKEN","operation":{"order":0,"source":".env.example"}}},{"spec":{"checked":false,"description":"","name":"Plain","required":true},"var":{"created":"2024-03-11T16:42:52.999507-04:00","key":"MSG","operation":{"order":0,"source":".env.example"}}},{"spec":{"checked":false,"description":"Some value","name":"Plain","required":false},"var":{"created":"2024-03-11T16:42:52.999508-04:00","key":"NAKED","operation":{"order":0,"source":".env.example"}}},{"spec":{"checked":false,"description":"Some name","name":"Plain","operation":{"order":0,"source":".env.example"},"required":true},"var":{"created":"2024-03-11T16:42:52.999506-04:00","key":"NAME"}},{"spec":{"checked":false,"description":"","name":"Plain","required":true},"var":{"created":"2024-03-11T16:42:52.999508-04:00","key":"USER","operation":{"order":0,"source":".env.example"}}},{"spec":{"checked":false,"description":"No idea what mode this is","name":"Plain","required":true},"var":{"created":"2024-03-11T16:42:52.999508-04:00","key":"COMMAND_MODE","operation":{"order":0,"source":".env.example"}}},{"spec":{"checked":false,"description":"Working directory","name":"Plain","required":false},"var":{"created":"2024-03-11T16:42:52.999509-04:00","key":"PWD","operation":{"order":0,"source":".env.example"}}}],"load_2":[{"value":{"original":"Luna","status":"","operation":{"order":0,"source":".env"}},"var":{"created":"2024-03-11T16:42:52.99951-04:00","key":"NAME"}}],"reconcile_12":[{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:43:00.194861-04:00","key":"__","operation":null}}],"reconcile_3":[{"value":{"status":"UNRESOLVED"},"var":{"created":"2024-03-11T16:42:52.999659-04:00","key":"MSG","operation":null}},{"value":{"status":"UNRESOLVED"},"var":{"created":"2024-03-11T16:42:52.999659-04:00","key":"NAKED","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999648-04:00","key":"INFOPATH","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.99964-04:00","key":"ELECTRON_RUN_AS_NODE","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999641-04:00","key":"TERMINFO","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999647-04:00","key":"HOMEBREW_CELLAR","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999644-04:00","key":"PATH","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999646-04:00","key":"GOPATH","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999651-04:00","key":"__CF_USER_TEXT_ENCODING","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999657-04:00","key":"APPLICATION_INSIGHTS_NO_DIAGNOSTIC_CHANNEL","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.99964-04:00","key":"BEGIN_INSTALL","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999641-04:00","key":"VSCODE_CWD","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999643-04:00","key":"XPC_SERVICE_NAME","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999648-04:00","key":"VSCODE_AMD_ENTRYPOINT","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999649-04:00","key":"LS_COLORS","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999649-04:00","key":"OLDPWD","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999654-04:00","key":"SSH_AUTH_SOCK","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999654-04:00","key":"VSCODE_PID","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999642-04:00","key":"VSCODE_CRASH_REPORTER_PROCESS_TYPE","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999642-04:00","key":"VSCODE_HANDLES_UNCAUGHT_ERRORS","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999644-04:00","key":"TMPDIR","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999655-04:00","key":"VSCODE_NLS_CONFIG","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999657-04:00","key":"LSCOLORS","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999652-04:00","key":"INSTRUMENTATION_KEY","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999657-04:00","key":"HOMEBREW_REPOSITORY","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999644-04:00","key":"HOME","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999646-04:00","key":"WASMTIME_HOME","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999646-04:00","key":"BUF_TOKEN","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999645-04:00","key":"TREE_COLORS","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999647-04:00","key":"LOGNAME","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999655-04:00","key":"LC_ALL","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999652-04:00","key":"XPC_FLAGS","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999653-04:00","key":"MANPATH","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999655-04:00","key":"HOMEBREW_PREFIX","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999656-04:00","key":"_","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999658-04:00","key":"MallocNanoZone","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999647-04:00","key":"USE_GKE_GCLOUD_AUTH_PLUGIN","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999649-04:00","key":"KRAFTCLOUD_USER","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999651-04:00","key":"SHLVL","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999656-04:00","key":"TERM","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999658-04:00","key":"SHELL","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999641-04:00","key":"PAGER","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999643-04:00","key":"ASDF_DIR","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999652-04:00","key":"ORIGINAL_XDG_CURRENT_DESKTOP","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999656-04:00","key":"LESS","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999639-04:00","key":"WASI_SDK_PATH","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999642-04:00","key":"VSCODE_IPC_HOOK","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:52.999648-04:00","key":"__CFBundleIdentifier","operation":null}}],"reconcile_6":[{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:42:59.978152-04:00","key":"RUNME_ID","operation":null}}],"reconcile_9":[{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-11T16:43:00.153761-04:00","key":"INNER","operation":null}}],"update_10":[{"value":{"original":"Hello world, Seb!\r\nsourishkrout\r\n","status":""},"var":{"created":"2024-03-11T16:43:00.194798-04:00","key":"__","operation":{"order":0,"source":"[execution]"}}}],"update_13":[{"value":{"original":"01HQ64XAYM289P5DF7CS0EJ54N","status":""},"var":{"created":"2024-03-11T16:43:04.956159-04:00","key":"RUNME_ID","operation":{"order":0,"source":"[execution]"}}},{"value":{"original":"xterm-256color","status":""},"var":{"created":"2024-03-11T16:43:04.956161-04:00","key":"TERM","operation":{"order":0,"source":"[execution]"}}}],"update_15":[{"value":{"original":"LPT","status":"","operation":{"order":0,"source":"[execution]"}},"var":{"created":"2024-03-11T16:43:05.155121-04:00","key":"NAME"}}],"update_17":[{"value":{"original":"Hello world, LPT!\r\nsourishkrout\r\n","status":""},"var":{"created":"2024-03-11T16:43:05.209067-04:00","key":"__","operation":{"order":0,"source":"[execution]"}}}],"update_19":[{"value":{"original":"01HQ64XAYM289P5DF7CS0EJ54N","status":""},"var":{"created":"2024-03-11T16:43:10.231094-04:00","key":"RUNME_ID","operation":{"order":0,"source":"[execution]"}}},{"value":{"original":"xterm-256color","status":""},"var":{"created":"2024-03-11T16:43:10.231096-04:00","key":"TERM","operation":{"order":0,"source":"[execution]"}}}],"update_21":[{"value":{"original":"Loon","status":"","operation":{"order":0,"source":"[execution]"}},"var":{"created":"2024-03-11T16:43:10.459397-04:00","key":"NAME"}}],"update_23":[{"value":{"original":"Hello world, Loon!\r\nsourishkrout\r\n","status":""},"var":{"created":"2024-03-11T16:43:10.525491-04:00","key":"__","operation":{"order":0,"source":"[execution]"}}}],"update_4":[{"value":{"original":"01HQ64XAYM289P5DF7CS0EJ54N","status":""},"var":{"created":"2024-03-11T16:42:59.978091-04:00","key":"RUNME_ID","operation":{"order":0,"source":"[execution]"}}},{"value":{"original":"xterm-256color","status":""},"var":{"created":"2024-03-11T16:42:59.978093-04:00","key":"TERM","operation":{"order":0,"source":"[execution]"}}}],"update_7":[{"value":{"original":"123","status":""},"var":{"created":"2024-03-11T16:43:00.153679-04:00","key":"NAKED","operation":{"order":0,"source":"[execution]"}}},{"value":{"original":"Seb","status":"","operation":{"order":0,"source":"[execution]"}},"var":{"created":"2024-03-11T16:43:00.153677-04:00","key":"NAME"}},{"value":{"original":"nested","status":""},"var":{"created":"2024-03-11T16:43:00.153679-04:00","key":"INNER","operation":{"order":0,"source":"[execution]"}}},{"value":{"original":"Hello world","status":""},"var":{"created":"2024-03-11T16:43:00.153679-04:00","key":"MSG","operation":{"order":0,"source":"[execution]"}}}]}`
		query := `query ResolveOwlSnapshot($insecure: Boolean = false, $load_0: [VariableInput]!, $load_1: [VariableInput]!, $load_2: [VariableInput]!, $reconcile_3: [VariableInput]!, $update_4: [VariableInput]!, $reconcile_6: [VariableInput]!, $update_7: [VariableInput]!, $reconcile_9: [VariableInput]!, $update_10: [VariableInput]!, $reconcile_12: [VariableInput]!, $update_13: [VariableInput]!, $update_15: [VariableInput]!, $update_17: [VariableInput]!, $update_19: [VariableInput]!, $update_21: [VariableInput]!, $update_23: [VariableInput]!) {
  environment {
    load(vars: $load_0, hasSpecs: false) {
      load(vars: $load_1, hasSpecs: true) {
        load(vars: $load_2, hasSpecs: false) {
          reconcile(vars: $reconcile_3, hasSpecs: true) {
            update(vars: $update_4, hasSpecs: false) {
              reconcile(vars: $reconcile_6, hasSpecs: true) {
                update(vars: $update_7, hasSpecs: false) {
                  reconcile(vars: $reconcile_9, hasSpecs: true) {
                    update(vars: $update_10, hasSpecs: false) {
                      reconcile(vars: $reconcile_12, hasSpecs: true) {
                        update(vars: $update_13, hasSpecs: false) {
                          update(vars: $update_15, hasSpecs: false) {
                            update(vars: $update_17, hasSpecs: false) {
                              update(vars: $update_19, hasSpecs: false) {
                                update(vars: $update_21, hasSpecs: false) {
                                  update(vars: $update_23, hasSpecs: false) {
                                    validate {
                                      Opaque(insecure: $insecure, keys: ["INSTRUMENTATION_KEY", "MANPATH", "ASDF_DIR", "VSCODE_CWD", "LSCOLORS", "INNER", "__", "USE_GKE_GCLOUD_AUTH_PLUGIN", "TERM", "WASI_SDK_PATH", "INFOPATH", "APPLICATION_INSIGHTS_NO_DIAGNOSTIC_CHANNEL", "SHELL", "VSCODE_AMD_ENTRYPOINT", "LS_COLORS", "VSCODE_CRASH_REPORTER_PROCESS_TYPE", "TMPDIR", "BUF_TOKEN", "TREE_COLORS", "LESS", "HOMEBREW_CELLAR", "__CF_USER_TEXT_ENCODING", "LOGNAME", "VSCODE_NLS_CONFIG", "HOME", "GOPATH", "OLDPWD", "MallocNanoZone", "KRAFTCLOUD_USER", "BEGIN_INSTALL", "HOMEBREW_REPOSITORY", "VSCODE_IPC_HOOK", "ELECTRON_RUN_AS_NODE", "VSCODE_HANDLES_UNCAUGHT_ERRORS", "XPC_SERVICE_NAME", "PATH", "WASMTIME_HOME", "LC_ALL", "_", "HOMEBREW_PREFIX", "XPC_FLAGS", "__CFBundleIdentifier", "TERMINFO", "VSCODE_PID", "SHLVL", "PAGER", "ORIGINAL_XDG_CURRENT_DESKTOP", "RUNME_ID", "SSH_AUTH_SOCK"]) {
                                        spec
                                        sensitive
                                        mask
                                        Password(insecure: $insecure, keys: ["KRAFTCLOUD_TOKEN"]) {
                                          spec
                                          sensitive
                                          mask
                                          Plain(insecure: $insecure, keys: ["MSG", "OPENAI_ORG_ID", "NAME", "USER", "PWD", "COMMAND_MODE", "NAKED"]) {
                                            spec
                                            sensitive
                                            mask
                                            Secret(insecure: $insecure, keys: ["OPENAI_API_KEY"]) {
                                              spec
                                              sensitive
                                              mask
                                              done {
                                                render {
                                                  snapshot(insecure: $insecure) {
                                                    var {
                                                      key
                                                      origin
                                                      created
                                                      updated
                                                      operation {
                                                        source
                                                      }
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
                                                    errors {
                                                      code
                                                      message
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

		require.False(t, result.HasErrors())

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
	})

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

	t.Run("query envs without specs", func(t *testing.T) {
		var vars map[string]interface{}
		err := json.Unmarshal([]byte(`{"insecure":false,"load_0":[{"var":{"key":"GOPATH","created":"2024-03-02T13:25:01.270468-05:00","operation":{"source":"[system]"}},"spec":{"checked":false,"name":"Opaque","required":false},"value":{"resolved":"/Users/sourishkrout/go","status":""}},{"var":{"created":"2024-03-02T13:25:01.270469-05:00","key":"HOME","operation":{"source":"[system]"}},"spec":{"checked":false,"name":"Secret","required":true},"value":{"resolved":"/Users/sourishkrout","status":""}},{"var":{"created":"2024-03-02T13:25:01.270471-05:00","key":"HOMEBREW_REPOSITORY","operation":{"source":"[system]"}},"spec":{"checked":false,"name":"Plain","required":false},"value":{"resolved":"/opt/homebrew","status":""}}]}`), &vars)
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
          Secret(keys: ["HOME"]) {
            spec
            sensitive
            mask
            Plain(keys: ["HOMEBREW_REPOSITORY"]) {
              spec
              sensitive
              mask
              done {
                render {
                  snapshot(insecure: $insecure) {
                    var {
                      key
                      created
                      updated
                      operation {
                        source
                      }
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
                    errors {
                      code
                      message
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
}`,
			VariableValues: vars,
		})
		require.False(t, result.HasErrors())
		fmt.Println(result.Errors)

		render, err := extractDataKey(result.Data, "render")
		require.NoError(t, err)
		require.NotNil(t, render)

		b, err := yaml.Marshal(render)
		// b, err := json.MarshalIndent(result, "", " ")
		require.NoError(t, err)
		fmt.Println(string(b))
		require.NotNil(t, b)
	})
}

func Test_Graph_Update(t *testing.T) {
	var vars map[string]interface{}
	err := json.Unmarshal([]byte(`{"insecure":true,"load_0":[{"value":{"original":"fi=00:mi=00:mh=00:ln=01;36:or=01;31:di=01;34:ow=04;01;34:st=34:tw=04;34:pi=01;33:so=01;33:do=01;33:bd=01;33:cd=01;33:su=01;35:sg=01;35:ca=01;35:ex=01;32","status":""},"var":{"created":"2024-03-10T20:48:58.072091-04:00","key":"LS_COLORS","operation":null}},{"value":{"original":"0","status":""},"var":{"created":"2024-03-10T20:48:58.072092-04:00","key":"MallocNanoZone","operation":null}},{"value":{"original":"0","status":""},"var":{"created":"2024-03-10T20:48:58.072102-04:00","key":"SHLVL","operation":null}},{"value":{"original":"/var/folders/c3/5r0t1nzs7sbfpxjgbc6n3ss40000gn/T/","status":""},"var":{"created":"2024-03-10T20:48:58.072104-04:00","key":"TMPDIR","operation":null}},{"value":{"original":"/Users/sourishkrout/.wasmtime","status":""},"var":{"created":"2024-03-10T20:48:58.072111-04:00","key":"WASMTIME_HOME","operation":null}},{"value":{"original":"1","status":""},"var":{"created":"2024-03-10T20:48:58.072115-04:00","key":"APPLICATION_INSIGHTS_NO_DIAGNOSTIC_CHANNEL","operation":null}},{"value":{"original":"/Users/sourishkrout/go","status":""},"var":{"created":"2024-03-10T20:48:58.072081-04:00","key":"GOPATH","operation":null}},{"value":{"original":"achristian","status":""},"var":{"created":"2024-03-10T20:48:58.072087-04:00","key":"KRAFTCLOUD_USER","operation":null}},{"value":{"original":"/private/tmp/com.apple.launchd.WJncT7ZrHW/Listeners","status":""},"var":{"created":"2024-03-10T20:48:58.072103-04:00","key":"SSH_AUTH_SOCK","operation":null}},{"value":{"original":"sourishkrout","status":""},"var":{"created":"2024-03-10T20:48:58.072107-04:00","key":"USER","operation":null}},{"value":{"original":"True","status":""},"var":{"created":"2024-03-10T20:48:58.072107-04:00","key":"USE_GKE_GCLOUD_AUTH_PLUGIN","operation":null}},{"value":{"original":"/","status":""},"var":{"created":"2024-03-10T20:48:58.072109-04:00","key":"VSCODE_CWD","operation":null}},{"value":{"original":"less","status":""},"var":{"created":"2024-03-10T20:48:58.072095-04:00","key":"PAGER","operation":null}},{"value":{"original":"/Users/sourishkrout/Library/Application Support/Code/1.87-main.sock","status":""},"var":{"created":"2024-03-10T20:48:58.07211-04:00","key":"VSCODE_IPC_HOOK","operation":null}},{"value":{"original":"{\"locale\":\"en-us\",\"osLocale\":\"en-us\",\"availableLanguages\":{},\"_languagePackSupport\":true}","status":""},"var":{"created":"2024-03-10T20:48:58.07211-04:00","key":"VSCODE_NLS_CONFIG","operation":null}},{"value":{"original":"com.microsoft.VSCode","status":""},"var":{"created":"2024-03-10T20:48:58.072113-04:00","key":"__CFBundleIdentifier","operation":null}},{"value":{"original":"0x1F5:0x0:0x0","status":""},"var":{"created":"2024-03-10T20:48:58.072114-04:00","key":"__CF_USER_TEXT_ENCODING","operation":null}},{"value":{"original":"/opt/homebrew","status":""},"var":{"created":"2024-03-10T20:48:58.072082-04:00","key":"HOMEBREW_PREFIX","operation":null}},{"value":{"original":"/opt/homebrew/share/info:","status":""},"var":{"created":"2024-03-10T20:48:58.072085-04:00","key":"INFOPATH","operation":null}},{"value":{"original":"sourishkrout","status":""},"var":{"created":"2024-03-10T20:48:58.072089-04:00","key":"LOGNAME","operation":null}},{"value":{"original":"0x0","status":""},"var":{"created":"2024-03-10T20:48:58.072112-04:00","key":"XPC_FLAGS","operation":null}},{"value":{"original":"/opt/homebrew/opt/asdf/libexec","status":""},"var":{"created":"2024-03-10T20:48:58.072076-04:00","key":"ASDF_DIR","operation":null}},{"value":{"original":"/Users/sourishkrout","status":""},"var":{"created":"2024-03-10T20:48:58.072081-04:00","key":"HOME","operation":null}},{"value":{"original":"ExGxDxDxCxDxDxFxFxexEx","status":""},"var":{"created":"2024-03-10T20:48:58.072089-04:00","key":"LSCOLORS","operation":null}},{"value":{"original":"/","status":""},"var":{"created":"2024-03-10T20:48:58.072096-04:00","key":"PWD","operation":null}},{"value":{"original":"extensionHost","status":""},"var":{"created":"2024-03-10T20:48:58.072108-04:00","key":"VSCODE_CRASH_REPORTER_PROCESS_TYPE","operation":null}},{"value":{"original":"true","status":""},"var":{"created":"2024-03-10T20:48:58.072109-04:00","key":"VSCODE_HANDLES_UNCAUGHT_ERRORS","operation":null}},{"value":{"original":"89716","status":""},"var":{"created":"2024-03-10T20:48:58.072111-04:00","key":"VSCODE_PID","operation":null}},{"value":{"original":"/Applications/Visual Studio Code.app/Contents/MacOS/Electron","status":""},"var":{"created":"2024-03-10T20:48:58.072113-04:00","key":"_","operation":null}},{"value":{"original":"unix2003","status":""},"var":{"created":"2024-03-10T20:48:58.07208-04:00","key":"COMMAND_MODE","operation":null}},{"value":{"original":"-iRFXMx4","status":""},"var":{"created":"2024-03-10T20:48:58.072088-04:00","key":"LESS","operation":null}},{"value":{"original":"1","status":""},"var":{"created":"2024-03-10T20:48:58.072115-04:00","key":"ELECTRON_RUN_AS_NODE","operation":null}},{"value":{"original":"/opt/homebrew/share/man:/usr/share/man:/usr/local/share/man:/Users/sourishkrout/.cache/zsh4humans/v5/fzf/man:","status":""},"var":{"created":"2024-03-10T20:48:58.072092-04:00","key":"MANPATH","operation":null}},{"value":{"original":"/","status":""},"var":{"created":"2024-03-10T20:48:58.072093-04:00","key":"OLDPWD","operation":null}},{"value":{"original":"/Users/sourishkrout/.terminfo","status":""},"var":{"created":"2024-03-10T20:48:58.072104-04:00","key":"TERMINFO","operation":null}},{"value":{"original":"application.com.microsoft.VSCode.251091548.251091554","status":""},"var":{"created":"2024-03-10T20:48:58.072112-04:00","key":"XPC_SERVICE_NAME","operation":null}},{"value":{"original":"undefined","status":""},"var":{"created":"2024-03-10T20:48:58.072094-04:00","key":"ORIGINAL_XDG_CURRENT_DESKTOP","operation":null}},{"value":{"original":"/bin/zsh","status":""},"var":{"created":"2024-03-10T20:48:58.072102-04:00","key":"SHELL","operation":null}},{"value":{"original":"xterm-256color","status":""},"var":{"created":"2024-03-10T20:48:58.072104-04:00","key":"TERM","operation":null}},{"value":{"original":"fi=00:mi=00:mh=00:ln=01;36:or=01;31:di=01;34:ow=01;34:st=34:tw=34:pi=01;33:so=01;33:do=01;33:bd=01;33:cd=01;33:su=01;35:sg=01;35:ca=01;35:ex=01;32","status":""},"var":{"created":"2024-03-10T20:48:58.072106-04:00","key":"TREE_COLORS","operation":null}},{"value":{"original":"/Users/sourishkrout/Projects/stateful/2022Q4/wasi-sdk/dist/wasi-sdk-16.5ga0a342ac182c","status":""},"var":{"created":"2024-03-10T20:48:58.072111-04:00","key":"WASI_SDK_PATH","operation":null}},{"value":{"original":"/Users/sourishkrout/.begin","status":""},"var":{"created":"2024-03-10T20:48:58.072079-04:00","key":"BEGIN_INSTALL","operation":null}},{"value":{"original":"/opt/homebrew/Cellar","status":""},"var":{"created":"2024-03-10T20:48:58.072082-04:00","key":"HOMEBREW_CELLAR","operation":null}},{"value":{"original":"/opt/homebrew/share/google-cloud-sdk/bin:/Users/sourishkrout/.wasmtime/bin:/opt/homebrew/opt/libpq/bin:/Users/sourishkrout/go/bin:/Users/sourishkrout/.asdf/shims:/opt/homebrew/opt/asdf/libexec/bin:/Users/sourishkrout/bin:/opt/homebrew/bin:/opt/homebrew/sbin:/usr/local/bin:/System/Cryptexes/App/usr/bin:/var/run/com.apple.security.cryptexd/codex.system/bootstrap/usr/local/bin:/var/run/com.apple.security.cryptexd/codex.system/bootstrap/usr/bin:/var/run/com.apple.security.cryptexd/codex.system/bootstrap/usr/appleinternal/bin:/Library/Apple/usr/bin:/usr/bin:/bin:/usr/sbin:/sbin:/Users/sourishkrout/.cache/zsh4humans/v5/fzf/bin:/Applications/Postgres.app/Contents/Versions/16/bin","status":""},"var":{"created":"2024-03-10T20:48:58.072095-04:00","key":"PATH","operation":null}},{"value":{"original":"vs/workbench/api/node/extensionHostProcess","status":""},"var":{"created":"2024-03-10T20:48:58.072108-04:00","key":"VSCODE_AMD_ENTRYPOINT","operation":null}},{"value":{"original":"/opt/homebrew","status":""},"var":{"created":"2024-03-10T20:48:58.072083-04:00","key":"HOMEBREW_REPOSITORY","operation":null}},{"value":{"original":"en_US.UTF-8","status":""},"var":{"created":"2024-03-10T20:48:58.072087-04:00","key":"LC_ALL","operation":null}}],"load_1":[{"spec":{"checked":false,"description":"Some name","name":"Plain","required":true},"var":{"created":"2024-03-10T20:48:58.072167-04:00","key":"NAME","operation":null}},{"spec":{"checked":false,"description":"No idea what mode this is","name":"Plain","required":true},"var":{"created":"2024-03-10T20:48:58.072165-04:00","key":"COMMAND_MODE","operation":null}},{"spec":{"checked":false,"description":"User","name":"Plain","required":false},"var":{"created":"2024-03-10T20:48:58.072166-04:00","key":"USER","operation":null}},{"spec":{"checked":false,"description":"The message","name":"Plain","required":true},"var":{"created":"2024-03-10T20:48:58.072168-04:00","key":"MSG","operation":null}},{"spec":{"checked":false,"description":"Working directory","name":"Plain","required":false},"var":{"created":"2024-03-10T20:48:58.072167-04:00","key":"PWD","operation":null}},{"spec":{"checked":false,"description":"Some value","name":"Plain","required":false},"var":{"created":"2024-03-10T20:48:58.072167-04:00","key":"NAKED","operation":null}}],"reconcile_12":[{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:49:29.262762-04:00","key":"__","operation":null}}],"reconcile_3":[{"value":{"status":"UNRESOLVED"},"var":{"created":"2024-03-10T20:48:58.072317-04:00","key":"NAME","operation":null}},{"value":{"status":"UNRESOLVED"},"var":{"created":"2024-03-10T20:48:58.072318-04:00","key":"NAKED","operation":null}},{"value":{"status":"UNRESOLVED"},"var":{"created":"2024-03-10T20:48:58.072318-04:00","key":"MSG","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072307-04:00","key":"HOMEBREW_PREFIX","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072315-04:00","key":"TERM","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072316-04:00","key":"TREE_COLORS","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072316-04:00","key":"WASI_SDK_PATH","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072301-04:00","key":"TMPDIR","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072303-04:00","key":"LS_COLORS","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072304-04:00","key":"USE_GKE_GCLOUD_AUTH_PLUGIN","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072303-04:00","key":"KRAFTCLOUD_USER","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072308-04:00","key":"LOGNAME","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072312-04:00","key":"VSCODE_PID","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072313-04:00","key":"ELECTRON_RUN_AS_NODE","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.0723-04:00","key":"PATH","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072301-04:00","key":"SHLVL","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072301-04:00","key":"WASMTIME_HOME","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072307-04:00","key":"__CF_USER_TEXT_ENCODING","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072316-04:00","key":"HOMEBREW_CELLAR","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072298-04:00","key":"VSCODE_AMD_ENTRYPOINT","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072299-04:00","key":"HOMEBREW_REPOSITORY","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072306-04:00","key":"SSH_AUTH_SOCK","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.07231-04:00","key":"VSCODE_CRASH_REPORTER_PROCESS_TYPE","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072312-04:00","key":"_","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072313-04:00","key":"LSCOLORS","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072299-04:00","key":"LC_ALL","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072302-04:00","key":"GOPATH","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072308-04:00","key":"ASDF_DIR","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072316-04:00","key":"BEGIN_INSTALL","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072306-04:00","key":"VSCODE_NLS_CONFIG","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072307-04:00","key":"VSCODE_IPC_HOOK","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072309-04:00","key":"HOME","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072311-04:00","key":"VSCODE_HANDLES_UNCAUGHT_ERRORS","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072312-04:00","key":"LESS","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072315-04:00","key":"MANPATH","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072302-04:00","key":"APPLICATION_INSIGHTS_NO_DIAGNOSTIC_CHANNEL","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072304-04:00","key":"VSCODE_CWD","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072305-04:00","key":"PAGER","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072314-04:00","key":"XPC_SERVICE_NAME","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072315-04:00","key":"SHELL","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072306-04:00","key":"__CFBundleIdentifier","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072308-04:00","key":"XPC_FLAGS","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072314-04:00","key":"TERMINFO","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072317-04:00","key":"ORIGINAL_XDG_CURRENT_DESKTOP","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.0723-04:00","key":"MallocNanoZone","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072309-04:00","key":"INFOPATH","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:48:58.072313-04:00","key":"OLDPWD","operation":null}}],"reconcile_6":[{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:49:29.040802-04:00","key":"RUNME_ID","operation":null}}],"reconcile_9":[{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-10T20:49:29.228105-04:00","key":"INNER","operation":null}}],"update_10":[{"value":{"original":"Hello world, LPT!\r\nsourishkrout\r\n","status":""},"var":{"created":"2024-03-10T20:49:29.262703-04:00","key":"__","operation":null}}],"update_13":[{"value":{"original":"01HRF7RKVNFZCZ9P8GH7CNHZSY","status":""},"var":{"created":"2024-03-10T20:49:36.136807-04:00","key":"RUNME_ID","operation":null}},{"value":{"original":"xterm-256color","status":""},"var":{"created":"2024-03-10T20:49:36.136809-04:00","key":"TERM","operation":null}}],"update_17":[{"value":{"original":"Hello world\r\n123\r\nsourishkrout\r\nLPT\r\n/Users/sourishkrout/Projects/stateful/oss/vscode-runme/examples\r\nnested\r\n","status":""},"var":{"created":"2024-03-10T20:49:36.414365-04:00","key":"__","operation":null}}],"update_4":[{"value":{"original":"01HQ64XAYM289P5DF7CS0EJ54N","status":""},"var":{"created":"2024-03-10T20:49:29.040753-04:00","key":"RUNME_ID","operation":null}},{"value":{"original":"xterm-256color","status":""},"var":{"created":"2024-03-10T20:49:29.040755-04:00","key":"TERM","operation":null}}],"update_7":[{"value":{"original":"Hello world","status":""},"var":{"created":"2024-03-10T20:49:29.228017-04:00","key":"MSG","operation":null}},{"value":{"original":"nested","status":""},"var":{"created":"2024-03-10T20:49:29.228019-04:00","key":"INNER","operation":null}},{"value":{"original":"LPT","status":""},"var":{"created":"2024-03-10T20:49:29.228019-04:00","key":"NAME","operation":null}},{"value":{"original":"123","status":""},"var":{"created":"2024-03-10T20:49:29.22802-04:00","key":"NAKED","operation":null}}]}`), &vars)
	require.NoError(t, err)

	fakeQuery := `query ResolveOwlSnapshot($insecure: Boolean = false, $load_0: [VariableInput]!, $load_1: [VariableInput]!, $reconcile_3: [VariableInput]!, $update_4: [VariableInput]!, $reconcile_6: [VariableInput]!, $update_7: [VariableInput]!, $reconcile_9: [VariableInput]!, $update_10: [VariableInput]!, $reconcile_12: [VariableInput]!, $update_13: [VariableInput]!, $update_17: [VariableInput]!) {
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
                              spec
                              sensitive
                              mask
                              Plain(insecure: $insecure, keys: ["MSG", "PWD", "NAKED", "NAME", "USER", "COMMAND_MODE", "OPENAI_ORG_ID"]) {
                                spec
                                sensitive
                                mask
                                Secret(insecure: $insecure, keys: ["OPENAI_API_KEY", "KRAFTCLOUD_TOKEN"]) {
                                  spec
                                  sensitive
                                  mask
                                  done {
                                    render {
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
                                          checked
                                        }
                                        errors {
                                          code
                                          message
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
    }
  }
}`

	result := graphql.Do(graphql.Params{
		Schema:         Schema,
		RequestString:  fakeQuery,
		VariableValues: vars,
	})

	require.False(t, result.HasErrors())

	render, err := extractDataKey(result.Data, "render")
	require.NoError(t, err)
	require.NotNil(t, render)

	// b, err := yaml.Marshal(render)
	// b, err := json.MarshalIndent(result, "", " ")
	// require.NoError(t, err)
	// fmt.Println(string(b))
	// require.NotNil(t, b)
}

func Test_Graph_Required(t *testing.T) {
	t.Run("validate simple environment", func(t *testing.T) {
		var vars map[string]interface{}
		err := json.Unmarshal([]byte(`{"load_0":[{"var":{"key":"GOPATH","created":"2024-03-02T13:25:01.270468-05:00","operation":{"order":0,"source":".env.example"}},"spec":{"checked":false,"name":"Opaque","required":true},"value":{"status":"UNRESOLVED"}},{"var":{"key":"HOME","created":"2024-03-02T13:25:01.270469-05:00","operation":{"order":0,"source":".env.example"}},"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"/Users/sourishkrout","status":""}},{"var":{"key":"HOMEBREW_REPOSITORY","created":"2024-03-02T13:25:01.270471-05:00","operation":{"order":0,"source":".env.example"}},"spec":{"checked":false,"name":"Opaque","required":false},"value":{"original":"/opt/homebrew","status":""}}]}`), &vars)
		require.NoError(t, err)

		result := graphql.Do(graphql.Params{
			Schema: Schema,
			RequestString: `query ResolveOwlSnapshot($insecure: Boolean = false, $load_0: [VariableInput]!) {
  environment {
    load(vars: $load_0, hasSpecs: true) {
      validate {
        Opaque(insecure: $insecure, keys: ["GOPATH", "HOME", "HOMEBREW_REPOSITORY"]) {
          spec
          sensitive
          mask
          errors {
            code
            message
          }
          done {
            render {
              snapshot(insecure: $insecure) {
                var {
                  key
                  origin
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
                errors {
                  code
                  message
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
		fmt.Println(result.Errors)
		require.False(t, result.HasErrors())

		validate, err := extractDataKey(result.Data, "validate")
		require.NoError(t, err)
		require.NotNil(t, validate)

		b, err := yaml.Marshal(validate)
		// b, err := json.MarshalIndent(result, "", " ")
		require.NoError(t, err)
		fmt.Println(string(b))
		require.NotNil(t, b)
	})
}

func Test_Graph_LackOfOperation(t *testing.T) {
	// todo(sebastian): prevent panic; needs more work
	t.Run("MSG does not have operation associate", func(t *testing.T) {
		var vars map[string]interface{}
		err := json.Unmarshal([]byte(`{"insecure":false,"load_0":[{"value":{"original":"/opt/homebrew/share/man:/usr/share/man:/usr/local/share/man:/Users/sourishkrout/.cache/zsh4humans/v5/fzf/man:","status":""},"var":{"created":"2024-03-12T20:45:00.077665-04:00","key":"MANPATH","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"less","status":""},"var":{"created":"2024-03-12T20:45:00.077668-04:00","key":"PAGER","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"application.com.microsoft.VSCode.251091548.251091554","status":""},"var":{"created":"2024-03-12T20:45:00.07768-04:00","key":"XPC_SERVICE_NAME","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/Users/sourishkrout/.begin","status":""},"var":{"created":"2024-03-12T20:45:00.077653-04:00","key":"BEGIN_INSTALL","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/Users/sourishkrout/go","status":""},"var":{"created":"2024-03-12T20:45:00.077655-04:00","key":"GOPATH","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"fi=00:mi=00:mh=00:ln=01;36:or=01;31:di=01;34:ow=01;34:st=34:tw=34:pi=01;33:so=01;33:do=01;33:bd=01;33:cd=01;33:su=01;35:sg=01;35:ca=01;35:ex=01;32","status":""},"var":{"created":"2024-03-12T20:45:00.077675-04:00","key":"TREE_COLORS","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"sourishkrout","status":""},"var":{"created":"2024-03-12T20:45:00.077675-04:00","key":"USER","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"true","status":""},"var":{"created":"2024-03-12T20:45:00.077678-04:00","key":"VSCODE_HANDLES_UNCAUGHT_ERRORS","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"unix2003","status":""},"var":{"created":"2024-03-12T20:45:00.077654-04:00","key":"COMMAND_MODE","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"0","status":""},"var":{"created":"2024-03-12T20:45:00.077666-04:00","key":"MallocNanoZone","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/opt/homebrew/share/info:","status":""},"var":{"created":"2024-03-12T20:45:00.077659-04:00","key":"INFOPATH","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"vs/workbench/api/node/extensionHostProcess","status":""},"var":{"created":"2024-03-12T20:45:00.077676-04:00","key":"VSCODE_AMD_ENTRYPOINT","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"extensionHost","status":""},"var":{"created":"2024-03-12T20:45:00.077677-04:00","key":"VSCODE_CRASH_REPORTER_PROCESS_TYPE","operation":{"order":0,"source":"[system]"}}},{"value":{"status":""},"var":{"created":"2024-03-12T20:45:00.077683-04:00","key":"VSCODE_L10N_BUNDLE_LOCATION","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"cmxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxlFl","status":""},"var":{"created":"2024-03-12T20:45:00.07766-04:00","key":"KRAFTCLOUD_TOKEN","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/Users/sourishkrout/.wasmtime","status":""},"var":{"created":"2024-03-12T20:45:00.07768-04:00","key":"WASMTIME_HOME","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"0x0","status":""},"var":{"created":"2024-03-12T20:45:00.07768-04:00","key":"XPC_FLAGS","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"d8xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx188","status":""},"var":{"created":"2024-03-12T20:45:00.077654-04:00","key":"BUF_TOKEN","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"fi=00:mi=00:mh=00:ln=01;36:or=01;31:di=01;34:ow=04;01;34:st=34:tw=04;34:pi=01;33:so=01;33:do=01;33:bd=01;33:cd=01;33:su=01;35:sg=01;35:ca=01;35:ex=01;32","status":""},"var":{"created":"2024-03-12T20:45:00.077664-04:00","key":"LS_COLORS","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/","status":""},"var":{"created":"2024-03-12T20:45:00.077677-04:00","key":"VSCODE_CWD","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/Users/sourishkrout/Projects/stateful/2022Q4/wasi-sdk/dist/wasi-sdk-16.5ga0a342ac182c","status":""},"var":{"created":"2024-03-12T20:45:00.077679-04:00","key":"WASI_SDK_PATH","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"93046","status":""},"var":{"created":"2024-03-12T20:45:00.077679-04:00","key":"VSCODE_PID","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/Applications/Visual Studio Code.app/Contents/MacOS/Electron","status":""},"var":{"created":"2024-03-12T20:45:00.077681-04:00","key":"_","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/opt/homebrew/opt/asdf/libexec","status":""},"var":{"created":"2024-03-12T20:45:00.077651-04:00","key":"ASDF_DIR","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"sourishkrout","status":""},"var":{"created":"2024-03-12T20:45:00.077662-04:00","key":"LOGNAME","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/opt/homebrew/share/google-cloud-sdk/bin:/Users/sourishkrout/.wasmtime/bin:/opt/homebrew/opt/libpq/bin:/Users/sourishkrout/go/bin:/Users/sourishkrout/.asdf/shims:/opt/homebrew/opt/asdf/libexec/bin:/Users/sourishkrout/bin:/opt/homebrew/bin:/opt/homebrew/sbin:/usr/local/bin:/System/Cryptexes/App/usr/bin:/var/run/com.apple.security.cryptexd/codex.system/bootstrap/usr/local/bin:/var/run/com.apple.security.cryptexd/codex.system/bootstrap/usr/bin:/var/run/com.apple.security.cryptexd/codex.system/bootstrap/usr/appleinternal/bin:/Library/Apple/usr/bin:/usr/bin:/bin:/usr/sbin:/sbin:/Users/sourishkrout/.cache/zsh4humans/v5/fzf/bin:/Applications/Postgres.app/Contents/Versions/16/bin","status":""},"var":{"created":"2024-03-12T20:45:00.077668-04:00","key":"PATH","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"1","status":""},"var":{"created":"2024-03-12T20:45:00.077682-04:00","key":"ELECTRON_RUN_AS_NODE","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"en_US.UTF-8","status":""},"var":{"created":"2024-03-12T20:45:00.077661-04:00","key":"LC_ALL","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"undefined","status":""},"var":{"created":"2024-03-12T20:45:00.077667-04:00","key":"ORIGINAL_XDG_CURRENT_DESKTOP","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/var/folders/c3/5r0t1nzs7sbfpxjgbc6n3ss40000gn/T/","status":""},"var":{"created":"2024-03-12T20:45:00.077674-04:00","key":"TMPDIR","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/Users/sourishkrout","status":""},"var":{"created":"2024-03-12T20:45:00.077655-04:00","key":"HOME","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"xxxxxxxx-a41e-xxxx-xxxx-xxxxxxxxxxxx","status":""},"var":{"created":"2024-03-12T20:45:00.07766-04:00","key":"INSTRUMENTATION_KEY","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"achristian","status":""},"var":{"created":"2024-03-12T20:45:00.077661-04:00","key":"KRAFTCLOUD_USER","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"0x1F5:0x0:0x0","status":""},"var":{"created":"2024-03-12T20:45:00.077682-04:00","key":"__CF_USER_TEXT_ENCODING","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"sk-Kxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxq","status":""},"var":{"created":"2024-03-12T20:45:00.077666-04:00","key":"OPENAI_API_KEY","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/opt/homebrew","status":""},"var":{"created":"2024-03-12T20:45:00.077657-04:00","key":"HOMEBREW_REPOSITORY","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/Users/sourishkrout/.terminfo","status":""},"var":{"created":"2024-03-12T20:45:00.077673-04:00","key":"TERMINFO","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/opt/homebrew/Cellar","status":""},"var":{"created":"2024-03-12T20:45:00.077656-04:00","key":"HOMEBREW_CELLAR","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"org-tmxxxxxxxxxxxxxxxxxxxxk0","status":""},"var":{"created":"2024-03-12T20:45:00.077667-04:00","key":"OPENAI_ORG_ID","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"com.microsoft.VSCode","status":""},"var":{"created":"2024-03-12T20:45:00.077681-04:00","key":"__CFBundleIdentifier","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/opt/homebrew","status":""},"var":{"created":"2024-03-12T20:45:00.077656-04:00","key":"HOMEBREW_PREFIX","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"ExGxDxDxCxDxDxFxFxexEx","status":""},"var":{"created":"2024-03-12T20:45:00.077663-04:00","key":"LSCOLORS","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/bin/zsh","status":""},"var":{"created":"2024-03-12T20:45:00.077671-04:00","key":"SHELL","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"0","status":""},"var":{"created":"2024-03-12T20:45:00.077672-04:00","key":"SHLVL","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/private/tmp/com.apple.launchd.WJncT7ZrHW/Listeners","status":""},"var":{"created":"2024-03-12T20:45:00.077672-04:00","key":"SSH_AUTH_SOCK","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"1","status":""},"var":{"created":"2024-03-12T20:45:00.077682-04:00","key":"APPLICATION_INSIGHTS_NO_DIAGNOSTIC_CHANNEL","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"-iRFXMx4","status":""},"var":{"created":"2024-03-12T20:45:00.077662-04:00","key":"LESS","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/","status":""},"var":{"created":"2024-03-12T20:45:00.077666-04:00","key":"OLDPWD","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"xterm-256color","status":""},"var":{"created":"2024-03-12T20:45:00.077673-04:00","key":"TERM","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"{\"locale\":\"en-us\",\"osLocale\":\"en-us\",\"availableLanguages\":{},\"_languagePackSupport\":true}","status":""},"var":{"created":"2024-03-12T20:45:00.077678-04:00","key":"VSCODE_NLS_CONFIG","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/","status":""},"var":{"created":"2024-03-12T20:45:00.077669-04:00","key":"PWD","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"True","status":""},"var":{"created":"2024-03-12T20:45:00.077676-04:00","key":"USE_GKE_GCLOUD_AUTH_PLUGIN","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/Users/sourishkrout/Library/Application Support/Code/1.87-main.sock","status":""},"var":{"created":"2024-03-12T20:45:00.077678-04:00","key":"VSCODE_IPC_HOOK","operation":{"order":0,"source":"[system]"}}}],"load_1":[{"spec":{"checked":false,"description":"Some value","name":"Plain","required":false},"var":{"created":"2024-03-12T20:45:00.077717-04:00","key":"NAKED","operation":{"order":0,"source":".env.example"}}},{"spec":{"checked":false,"description":"Some name","name":"Plain","required":true},"var":{"created":"2024-03-12T20:45:00.077718-04:00","key":"NAME","operation":{"order":0,"source":".env.example"}}},{"spec":{"checked":false,"description":"No idea what mode this is","name":"Plain","required":true},"var":{"created":"2024-03-12T20:45:00.07772-04:00","key":"COMMAND_MODE","operation":{"order":0,"source":".env.example"}}},{"spec":{"checked":false,"description":"","name":"Plain","required":true},"var":{"created":"2024-03-12T20:45:00.07772-04:00","key":"MSG","operation":{"order":0,"source":".env.example"}}},{"spec":{"checked":false,"description":"Working directory","name":"Plain","required":false},"var":{"created":"2024-03-12T20:45:00.077718-04:00","key":"PWD","operation":{"order":0,"source":".env.example"}}},{"spec":{"checked":false,"description":"Your OpenAI API key matching the org","name":"Secret","required":true},"var":{"created":"2024-03-12T20:45:00.077719-04:00","key":"OPENAI_API_KEY","operation":{"order":0,"source":".env.example"}}},{"spec":{"checked":false,"description":"This is secret","name":"Password","required":true},"var":{"created":"2024-03-12T20:45:00.077719-04:00","key":"KRAFTCLOUD_TOKEN","operation":{"order":0,"source":".env.example"}}},{"spec":{"checked":false,"description":"","name":"Plain","required":true},"var":{"created":"2024-03-12T20:45:00.077719-04:00","key":"USER","operation":{"order":0,"source":".env.example"}}},{"spec":{"checked":false,"description":"Your OpenAI org identifier","name":"Plain","required":true},"var":{"created":"2024-03-12T20:45:00.07772-04:00","key":"OPENAI_ORG_ID","operation":{"order":0,"source":".env.example"}}}],"load_2":[{"value":{"original":"Luna","status":""},"var":{"created":"2024-03-12T20:45:00.077722-04:00","key":"NAME","operation":{"order":0,"source":".env"}}}],"reconcile_3":[{"value":{"status":"UNRESOLVED"},"var":{"created":"2024-03-12T20:45:00.077916-04:00","key":"MSG","operation":null}},{"value":{"status":"UNRESOLVED"},"var":{"created":"2024-03-12T20:45:00.077916-04:00","key":"NAKED","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077911-04:00","key":"__CFBundleIdentifier","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077911-04:00","key":"LSCOLORS","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077912-04:00","key":"SHLVL","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077913-04:00","key":"LESS","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077903-04:00","key":"WASMTIME_HOME","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077902-04:00","key":"LS_COLORS","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077906-04:00","key":"ORIGINAL_XDG_CURRENT_DESKTOP","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077908-04:00","key":"KRAFTCLOUD_USER","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077913-04:00","key":"SSH_AUTH_SOCK","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077895-04:00","key":"GOPATH","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077898-04:00","key":"INFOPATH","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.0779-04:00","key":"XPC_FLAGS","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077902-04:00","key":"VSCODE_CWD","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077903-04:00","key":"_","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077903-04:00","key":"ASDF_DIR","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077908-04:00","key":"INSTRUMENTATION_KEY","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077915-04:00","key":"USE_GKE_GCLOUD_AUTH_PLUGIN","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077897-04:00","key":"MallocNanoZone","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077906-04:00","key":"TMPDIR","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077907-04:00","key":"HOME","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077912-04:00","key":"SHELL","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077914-04:00","key":"OLDPWD","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077914-04:00","key":"TERM","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077915-04:00","key":"VSCODE_IPC_HOOK","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077899-04:00","key":"VSCODE_AMD_ENTRYPOINT","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077915-04:00","key":"VSCODE_NLS_CONFIG","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077895-04:00","key":"TREE_COLORS","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077899-04:00","key":"VSCODE_CRASH_REPORTER_PROCESS_TYPE","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077904-04:00","key":"LOGNAME","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077905-04:00","key":"VSCODE_PID","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077905-04:00","key":"ELECTRON_RUN_AS_NODE","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077905-04:00","key":"LC_ALL","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.07791-04:00","key":"HOMEBREW_CELLAR","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077896-04:00","key":"XPC_SERVICE_NAME","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077901-04:00","key":"BUF_TOKEN","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.07791-04:00","key":"HOMEBREW_REPOSITORY","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077913-04:00","key":"APPLICATION_INSIGHTS_NO_DIAGNOSTIC_CHANNEL","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.0779-04:00","key":"VSCODE_L10N_BUNDLE_LOCATION","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077894-04:00","key":"PAGER","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077895-04:00","key":"BEGIN_INSTALL","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077896-04:00","key":"VSCODE_HANDLES_UNCAUGHT_ERRORS","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077902-04:00","key":"WASI_SDK_PATH","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077904-04:00","key":"PATH","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077909-04:00","key":"__CF_USER_TEXT_ENCODING","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.07791-04:00","key":"TERMINFO","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077894-04:00","key":"MANPATH","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077911-04:00","key":"HOMEBREW_PREFIX","operation":null}}],"reconcile_6":[{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:05.291937-04:00","key":"RUNME_ID","operation":null}}],"update_4":[{"value":{"original":"01HRAZTSXWC0NX5Y3DAK9ZVG64","status":""},"var":{"created":"2024-03-12T20:45:05.291876-04:00","key":"RUNME_ID","operation":{"order":0,"source":"[execution]"}}},{"value":{"original":"xterm-256color","status":""},"var":{"created":"2024-03-12T20:45:05.291877-04:00","key":"TERM","operation":{"order":0,"source":"[execution]"}}}]}`), &vars)
		require.NoError(t, err)

		result := graphql.Do(graphql.Params{
			Schema: Schema,
			RequestString: `query ResolveOwlSnapshot($insecure: Boolean = false, $load_0: [VariableInput]!, $load_1: [VariableInput]!, $load_2: [VariableInput]!, $reconcile_3: [VariableInput]!, $update_4: [VariableInput]!, $reconcile_6: [VariableInput]!) {
  environment {
    load(vars: $load_0, hasSpecs: false) {
      load(vars: $load_1, hasSpecs: true) {
        load(vars: $load_2, hasSpecs: false) {
          reconcile(vars: $reconcile_3, hasSpecs: true) {
            update(vars: $update_4, hasSpecs: false) {
              reconcile(vars: $reconcile_6, hasSpecs: true) {
                validate {
                  Opaque(insecure: $insecure, keys: ["VSCODE_NLS_CONFIG", "VSCODE_CRASH_REPORTER_PROCESS_TYPE", "WASI_SDK_PATH", "VSCODE_CWD", "OLDPWD", "INSTRUMENTATION_KEY", "SHELL", "PATH", "LS_COLORS", "MallocNanoZone", "LOGNAME", "_", "VSCODE_PID", "APPLICATION_INSIGHTS_NO_DIAGNOSTIC_CHANNEL", "VSCODE_HANDLES_UNCAUGHT_ERRORS", "GOPATH", "XPC_FLAGS", "VSCODE_IPC_HOOK", "TMPDIR", "LC_ALL", "LESS", "PAGER", "BUF_TOKEN", "HOMEBREW_REPOSITORY", "TERM", "MANPATH", "WASMTIME_HOME", "LSCOLORS", "USE_GKE_GCLOUD_AUTH_PLUGIN", "__CF_USER_TEXT_ENCODING", "BEGIN_INSTALL", "ORIGINAL_XDG_CURRENT_DESKTOP", "RUNME_ID", "KRAFTCLOUD_USER", "ASDF_DIR", "INFOPATH", "TERMINFO", "SSH_AUTH_SOCK", "VSCODE_AMD_ENTRYPOINT", "HOMEBREW_CELLAR", "VSCODE_L10N_BUNDLE_LOCATION", "HOMEBREW_PREFIX", "__CFBundleIdentifier", "HOME", "SHLVL", "XPC_SERVICE_NAME", "TREE_COLORS", "ELECTRON_RUN_AS_NODE"]) {
                    spec
                    sensitive
                    mask
                    Password(insecure: $insecure, keys: ["KRAFTCLOUD_TOKEN"]) {
                      spec
                      sensitive
                      mask
                      Plain(insecure: $insecure, keys: ["NAME", "MSG", "PWD", "USER", "NAKED", "COMMAND_MODE", "OPENAI_ORG_ID"]) {
                        spec
                        sensitive
                        mask
                        Secret(insecure: $insecure, keys: ["OPENAI_API_KEY"]) {
                          spec
                          sensitive
                          mask
                          done {
                            render {
                              snapshot(insecure: $insecure) {
                                var {
                                  key
                                  origin
                                  created
                                  updated
                                  operation {
                                    source
                                  }
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
                                errors {
                                  code
                                  message
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
}`,
			VariableValues: vars,
		})
		fmt.Println(result.Errors)
		require.False(t, result.HasErrors())

		validate, err := extractDataKey(result.Data, "validate")
		require.NoError(t, err)
		require.NotNil(t, validate)

		b, err := yaml.Marshal(validate)
		// b, err := json.MarshalIndent(result, "", " ")
		require.NoError(t, err)
		fmt.Println(string(b))
		require.NotNil(t, b)
	})
}

func Test_Graph_Sensitive(t *testing.T) {
	t.Run("return keys of vars with sensitive values", func(t *testing.T) {
		var vars map[string]interface{}
		err := json.Unmarshal([]byte(`{"insecure":false,"load_0":[{"value":{"original":"/opt/homebrew/share/man:/usr/share/man:/usr/local/share/man:/Users/sourishkrout/.cache/zsh4humans/v5/fzf/man:","status":""},"var":{"created":"2024-03-12T20:45:00.077665-04:00","key":"MANPATH","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"less","status":""},"var":{"created":"2024-03-12T20:45:00.077668-04:00","key":"PAGER","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"application.com.microsoft.VSCode.251091548.251091554","status":""},"var":{"created":"2024-03-12T20:45:00.07768-04:00","key":"XPC_SERVICE_NAME","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/Users/sourishkrout/.begin","status":""},"var":{"created":"2024-03-12T20:45:00.077653-04:00","key":"BEGIN_INSTALL","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/Users/sourishkrout/go","status":""},"var":{"created":"2024-03-12T20:45:00.077655-04:00","key":"GOPATH","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"fi=00:mi=00:mh=00:ln=01;36:or=01;31:di=01;34:ow=01;34:st=34:tw=34:pi=01;33:so=01;33:do=01;33:bd=01;33:cd=01;33:su=01;35:sg=01;35:ca=01;35:ex=01;32","status":""},"var":{"created":"2024-03-12T20:45:00.077675-04:00","key":"TREE_COLORS","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"sourishkrout","status":""},"var":{"created":"2024-03-12T20:45:00.077675-04:00","key":"USER","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"true","status":""},"var":{"created":"2024-03-12T20:45:00.077678-04:00","key":"VSCODE_HANDLES_UNCAUGHT_ERRORS","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"unix2003","status":""},"var":{"created":"2024-03-12T20:45:00.077654-04:00","key":"COMMAND_MODE","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"0","status":""},"var":{"created":"2024-03-12T20:45:00.077666-04:00","key":"MallocNanoZone","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/opt/homebrew/share/info:","status":""},"var":{"created":"2024-03-12T20:45:00.077659-04:00","key":"INFOPATH","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"vs/workbench/api/node/extensionHostProcess","status":""},"var":{"created":"2024-03-12T20:45:00.077676-04:00","key":"VSCODE_AMD_ENTRYPOINT","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"extensionHost","status":""},"var":{"created":"2024-03-12T20:45:00.077677-04:00","key":"VSCODE_CRASH_REPORTER_PROCESS_TYPE","operation":{"order":0,"source":"[system]"}}},{"value":{"status":""},"var":{"created":"2024-03-12T20:45:00.077683-04:00","key":"VSCODE_L10N_BUNDLE_LOCATION","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"cmxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxlFl","status":""},"var":{"created":"2024-03-12T20:45:00.07766-04:00","key":"KRAFTCLOUD_TOKEN","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/Users/sourishkrout/.wasmtime","status":""},"var":{"created":"2024-03-12T20:45:00.07768-04:00","key":"WASMTIME_HOME","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"0x0","status":""},"var":{"created":"2024-03-12T20:45:00.07768-04:00","key":"XPC_FLAGS","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"d8xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx188","status":""},"var":{"created":"2024-03-12T20:45:00.077654-04:00","key":"BUF_TOKEN","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"fi=00:mi=00:mh=00:ln=01;36:or=01;31:di=01;34:ow=04;01;34:st=34:tw=04;34:pi=01;33:so=01;33:do=01;33:bd=01;33:cd=01;33:su=01;35:sg=01;35:ca=01;35:ex=01;32","status":""},"var":{"created":"2024-03-12T20:45:00.077664-04:00","key":"LS_COLORS","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/","status":""},"var":{"created":"2024-03-12T20:45:00.077677-04:00","key":"VSCODE_CWD","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/Users/sourishkrout/Projects/stateful/2022Q4/wasi-sdk/dist/wasi-sdk-16.5ga0a342ac182c","status":""},"var":{"created":"2024-03-12T20:45:00.077679-04:00","key":"WASI_SDK_PATH","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"93046","status":""},"var":{"created":"2024-03-12T20:45:00.077679-04:00","key":"VSCODE_PID","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/Applications/Visual Studio Code.app/Contents/MacOS/Electron","status":""},"var":{"created":"2024-03-12T20:45:00.077681-04:00","key":"_","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/opt/homebrew/opt/asdf/libexec","status":""},"var":{"created":"2024-03-12T20:45:00.077651-04:00","key":"ASDF_DIR","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"sourishkrout","status":""},"var":{"created":"2024-03-12T20:45:00.077662-04:00","key":"LOGNAME","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/opt/homebrew/share/google-cloud-sdk/bin:/Users/sourishkrout/.wasmtime/bin:/opt/homebrew/opt/libpq/bin:/Users/sourishkrout/go/bin:/Users/sourishkrout/.asdf/shims:/opt/homebrew/opt/asdf/libexec/bin:/Users/sourishkrout/bin:/opt/homebrew/bin:/opt/homebrew/sbin:/usr/local/bin:/System/Cryptexes/App/usr/bin:/var/run/com.apple.security.cryptexd/codex.system/bootstrap/usr/local/bin:/var/run/com.apple.security.cryptexd/codex.system/bootstrap/usr/bin:/var/run/com.apple.security.cryptexd/codex.system/bootstrap/usr/appleinternal/bin:/Library/Apple/usr/bin:/usr/bin:/bin:/usr/sbin:/sbin:/Users/sourishkrout/.cache/zsh4humans/v5/fzf/bin:/Applications/Postgres.app/Contents/Versions/16/bin","status":""},"var":{"created":"2024-03-12T20:45:00.077668-04:00","key":"PATH","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"1","status":""},"var":{"created":"2024-03-12T20:45:00.077682-04:00","key":"ELECTRON_RUN_AS_NODE","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"en_US.UTF-8","status":""},"var":{"created":"2024-03-12T20:45:00.077661-04:00","key":"LC_ALL","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"undefined","status":""},"var":{"created":"2024-03-12T20:45:00.077667-04:00","key":"ORIGINAL_XDG_CURRENT_DESKTOP","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/var/folders/c3/5r0t1nzs7sbfpxjgbc6n3ss40000gn/T/","status":""},"var":{"created":"2024-03-12T20:45:00.077674-04:00","key":"TMPDIR","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/Users/sourishkrout","status":""},"var":{"created":"2024-03-12T20:45:00.077655-04:00","key":"HOME","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"xxxxxxxx-a41e-xxxx-xxxx-xxxxxxxxxxxx","status":""},"var":{"created":"2024-03-12T20:45:00.07766-04:00","key":"INSTRUMENTATION_KEY","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"achristian","status":""},"var":{"created":"2024-03-12T20:45:00.077661-04:00","key":"KRAFTCLOUD_USER","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"0x1F5:0x0:0x0","status":""},"var":{"created":"2024-03-12T20:45:00.077682-04:00","key":"__CF_USER_TEXT_ENCODING","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"sk-Kxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxq","status":""},"var":{"created":"2024-03-12T20:45:00.077666-04:00","key":"OPENAI_API_KEY","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/opt/homebrew","status":""},"var":{"created":"2024-03-12T20:45:00.077657-04:00","key":"HOMEBREW_REPOSITORY","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/Users/sourishkrout/.terminfo","status":""},"var":{"created":"2024-03-12T20:45:00.077673-04:00","key":"TERMINFO","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/opt/homebrew/Cellar","status":""},"var":{"created":"2024-03-12T20:45:00.077656-04:00","key":"HOMEBREW_CELLAR","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"org-tmxxxxxxxxxxxxxxxxxxxxk0","status":""},"var":{"created":"2024-03-12T20:45:00.077667-04:00","key":"OPENAI_ORG_ID","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"com.microsoft.VSCode","status":""},"var":{"created":"2024-03-12T20:45:00.077681-04:00","key":"__CFBundleIdentifier","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/opt/homebrew","status":""},"var":{"created":"2024-03-12T20:45:00.077656-04:00","key":"HOMEBREW_PREFIX","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"ExGxDxDxCxDxDxFxFxexEx","status":""},"var":{"created":"2024-03-12T20:45:00.077663-04:00","key":"LSCOLORS","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/bin/zsh","status":""},"var":{"created":"2024-03-12T20:45:00.077671-04:00","key":"SHELL","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"0","status":""},"var":{"created":"2024-03-12T20:45:00.077672-04:00","key":"SHLVL","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/private/tmp/com.apple.launchd.WJncT7ZrHW/Listeners","status":""},"var":{"created":"2024-03-12T20:45:00.077672-04:00","key":"SSH_AUTH_SOCK","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"1","status":""},"var":{"created":"2024-03-12T20:45:00.077682-04:00","key":"APPLICATION_INSIGHTS_NO_DIAGNOSTIC_CHANNEL","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"-iRFXMx4","status":""},"var":{"created":"2024-03-12T20:45:00.077662-04:00","key":"LESS","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/","status":""},"var":{"created":"2024-03-12T20:45:00.077666-04:00","key":"OLDPWD","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"xterm-256color","status":""},"var":{"created":"2024-03-12T20:45:00.077673-04:00","key":"TERM","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"{\"locale\":\"en-us\",\"osLocale\":\"en-us\",\"availableLanguages\":{},\"_languagePackSupport\":true}","status":""},"var":{"created":"2024-03-12T20:45:00.077678-04:00","key":"VSCODE_NLS_CONFIG","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/","status":""},"var":{"created":"2024-03-12T20:45:00.077669-04:00","key":"PWD","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"True","status":""},"var":{"created":"2024-03-12T20:45:00.077676-04:00","key":"USE_GKE_GCLOUD_AUTH_PLUGIN","operation":{"order":0,"source":"[system]"}}},{"value":{"original":"/Users/sourishkrout/Library/Application Support/Code/1.87-main.sock","status":""},"var":{"created":"2024-03-12T20:45:00.077678-04:00","key":"VSCODE_IPC_HOOK","operation":{"order":0,"source":"[system]"}}}],"load_1":[{"spec":{"checked":false,"description":"Some value","name":"Plain","required":false},"var":{"created":"2024-03-12T20:45:00.077717-04:00","key":"NAKED","operation":{"order":0,"source":".env.example"}}},{"spec":{"checked":false,"description":"Some name","name":"Plain","required":true},"var":{"created":"2024-03-12T20:45:00.077718-04:00","key":"NAME","operation":{"order":0,"source":".env.example"}}},{"spec":{"checked":false,"description":"No idea what mode this is","name":"Plain","required":true},"var":{"created":"2024-03-12T20:45:00.07772-04:00","key":"COMMAND_MODE","operation":{"order":0,"source":".env.example"}}},{"spec":{"checked":false,"description":"","name":"Plain","required":true},"var":{"created":"2024-03-12T20:45:00.07772-04:00","key":"MSG","operation":{"order":0,"source":".env.example"}}},{"spec":{"checked":false,"description":"Working directory","name":"Plain","required":false},"var":{"created":"2024-03-12T20:45:00.077718-04:00","key":"PWD","operation":{"order":0,"source":".env.example"}}},{"spec":{"checked":false,"description":"Your OpenAI API key matching the org","name":"Secret","required":true},"var":{"created":"2024-03-12T20:45:00.077719-04:00","key":"OPENAI_API_KEY","operation":{"order":0,"source":".env.example"}}},{"spec":{"checked":false,"description":"This is secret","name":"Password","required":true},"var":{"created":"2024-03-12T20:45:00.077719-04:00","key":"KRAFTCLOUD_TOKEN","operation":{"order":0,"source":".env.example"}}},{"spec":{"checked":false,"description":"","name":"Plain","required":true},"var":{"created":"2024-03-12T20:45:00.077719-04:00","key":"USER","operation":{"order":0,"source":".env.example"}}},{"spec":{"checked":false,"description":"Your OpenAI org identifier","name":"Plain","required":true},"var":{"created":"2024-03-12T20:45:00.07772-04:00","key":"OPENAI_ORG_ID","operation":{"order":0,"source":".env.example"}}}],"load_2":[{"value":{"original":"Luna","status":""},"var":{"created":"2024-03-12T20:45:00.077722-04:00","key":"NAME","operation":{"order":0,"source":".env"}}}],"reconcile_3":[{"value":{"status":"UNRESOLVED"},"var":{"created":"2024-03-12T20:45:00.077916-04:00","key":"MSG","operation":null}},{"value":{"status":"UNRESOLVED"},"var":{"created":"2024-03-12T20:45:00.077916-04:00","key":"NAKED","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077911-04:00","key":"__CFBundleIdentifier","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077911-04:00","key":"LSCOLORS","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077912-04:00","key":"SHLVL","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077913-04:00","key":"LESS","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077903-04:00","key":"WASMTIME_HOME","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077902-04:00","key":"LS_COLORS","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077906-04:00","key":"ORIGINAL_XDG_CURRENT_DESKTOP","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077908-04:00","key":"KRAFTCLOUD_USER","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077913-04:00","key":"SSH_AUTH_SOCK","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077895-04:00","key":"GOPATH","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077898-04:00","key":"INFOPATH","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.0779-04:00","key":"XPC_FLAGS","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077902-04:00","key":"VSCODE_CWD","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077903-04:00","key":"_","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077903-04:00","key":"ASDF_DIR","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077908-04:00","key":"INSTRUMENTATION_KEY","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077915-04:00","key":"USE_GKE_GCLOUD_AUTH_PLUGIN","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077897-04:00","key":"MallocNanoZone","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077906-04:00","key":"TMPDIR","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077907-04:00","key":"HOME","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077912-04:00","key":"SHELL","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077914-04:00","key":"OLDPWD","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077914-04:00","key":"TERM","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077915-04:00","key":"VSCODE_IPC_HOOK","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077899-04:00","key":"VSCODE_AMD_ENTRYPOINT","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077915-04:00","key":"VSCODE_NLS_CONFIG","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077895-04:00","key":"TREE_COLORS","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077899-04:00","key":"VSCODE_CRASH_REPORTER_PROCESS_TYPE","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077904-04:00","key":"LOGNAME","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077905-04:00","key":"VSCODE_PID","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077905-04:00","key":"ELECTRON_RUN_AS_NODE","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077905-04:00","key":"LC_ALL","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.07791-04:00","key":"HOMEBREW_CELLAR","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077896-04:00","key":"XPC_SERVICE_NAME","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077901-04:00","key":"BUF_TOKEN","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.07791-04:00","key":"HOMEBREW_REPOSITORY","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077913-04:00","key":"APPLICATION_INSIGHTS_NO_DIAGNOSTIC_CHANNEL","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.0779-04:00","key":"VSCODE_L10N_BUNDLE_LOCATION","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077894-04:00","key":"PAGER","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077895-04:00","key":"BEGIN_INSTALL","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077896-04:00","key":"VSCODE_HANDLES_UNCAUGHT_ERRORS","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077902-04:00","key":"WASI_SDK_PATH","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077904-04:00","key":"PATH","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077909-04:00","key":"__CF_USER_TEXT_ENCODING","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.07791-04:00","key":"TERMINFO","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077894-04:00","key":"MANPATH","operation":null}},{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:00.077911-04:00","key":"HOMEBREW_PREFIX","operation":null}}],"reconcile_6":[{"spec":{"checked":false,"description":"","name":"Opaque","required":false},"var":{"created":"2024-03-12T20:45:05.291937-04:00","key":"RUNME_ID","operation":null}}],"update_4":[{"value":{"original":"01HRAZTSXWC0NX5Y3DAK9ZVG64","status":""},"var":{"created":"2024-03-12T20:45:05.291876-04:00","key":"RUNME_ID","operation":{"order":0,"source":"[execution]"}}},{"value":{"original":"xterm-256color","status":""},"var":{"created":"2024-03-12T20:45:05.291877-04:00","key":"TERM","operation":{"order":0,"source":"[execution]"}}}]}`), &vars)
		require.NoError(t, err)

		result := graphql.Do(graphql.Params{
			Schema: Schema,
			RequestString: `query ResolveOwlSnapshot($insecure: Boolean = false, $load_0: [VariableInput]!, $load_1: [VariableInput]!, $load_2: [VariableInput]!, $reconcile_3: [VariableInput]!, $update_4: [VariableInput]!, $reconcile_6: [VariableInput]!) {
  environment {
    load(vars: $load_0, hasSpecs: false) {
      load(vars: $load_1, hasSpecs: true) {
        load(vars: $load_2, hasSpecs: false) {
          reconcile(vars: $reconcile_3, hasSpecs: true) {
            update(vars: $update_4, hasSpecs: false) {
              reconcile(vars: $reconcile_6, hasSpecs: true) {
                validate {
                  Opaque(insecure: $insecure, keys: ["VSCODE_NLS_CONFIG", "VSCODE_CRASH_REPORTER_PROCESS_TYPE", "WASI_SDK_PATH", "VSCODE_CWD", "OLDPWD", "INSTRUMENTATION_KEY", "SHELL", "PATH", "LS_COLORS", "MallocNanoZone", "LOGNAME", "_", "VSCODE_PID", "APPLICATION_INSIGHTS_NO_DIAGNOSTIC_CHANNEL", "VSCODE_HANDLES_UNCAUGHT_ERRORS", "GOPATH", "XPC_FLAGS", "VSCODE_IPC_HOOK", "TMPDIR", "LC_ALL", "LESS", "PAGER", "BUF_TOKEN", "HOMEBREW_REPOSITORY", "TERM", "MANPATH", "WASMTIME_HOME", "LSCOLORS", "USE_GKE_GCLOUD_AUTH_PLUGIN", "__CF_USER_TEXT_ENCODING", "BEGIN_INSTALL", "ORIGINAL_XDG_CURRENT_DESKTOP", "RUNME_ID", "KRAFTCLOUD_USER", "ASDF_DIR", "INFOPATH", "TERMINFO", "SSH_AUTH_SOCK", "VSCODE_AMD_ENTRYPOINT", "HOMEBREW_CELLAR", "VSCODE_L10N_BUNDLE_LOCATION", "HOMEBREW_PREFIX", "__CFBundleIdentifier", "HOME", "SHLVL", "XPC_SERVICE_NAME", "TREE_COLORS", "ELECTRON_RUN_AS_NODE"]) {
                    spec
                    sensitive
                    mask
                    Password(insecure: $insecure, keys: ["KRAFTCLOUD_TOKEN"]) {
                      spec
                      sensitive
                      mask
                      Plain(insecure: $insecure, keys: ["NAME", "MSG", "PWD", "USER", "NAKED", "COMMAND_MODE", "OPENAI_ORG_ID"]) {
                        spec
                        sensitive
                        mask
                        Secret(insecure: $insecure, keys: ["OPENAI_API_KEY"]) {
                          spec
                          sensitive
                          mask
                          done {
                            render {
                              sensitiveKeys {
                                var {
                                  key
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
		fmt.Println(result.Errors)
		require.False(t, result.HasErrors())

		render, err := extractDataKey(result.Data, "render")
		require.NoError(t, err)
		require.NotNil(t, render)

		b, err := yaml.Marshal(render)
		// b, err := json.MarshalIndent(result, "", " ")
		require.NoError(t, err)
		fmt.Println(string(b))
		require.NotNil(t, b)
	})
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
                              spec
                              sensitive
                              mask
                              Plain(insecure: $insecure, keys: ["MSG", "PWD", "NAKED", "NAME", "USER", "COMMAND_MODE", "OPENAI_ORG_ID"]) {
                                spec
                                sensitive
                                mask
                                Secret(insecure: $insecure, keys: ["OPENAI_API_KEY", "KRAFTCLOUD_TOKEN"]) {
                                  spec
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

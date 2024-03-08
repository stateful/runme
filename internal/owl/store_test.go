//go:build !windows

package owl

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_OperationSet(t *testing.T) {
	t.Parallel()

	t.Run("withOperation", func(t *testing.T) {
		opSet, err := NewOperationSet(WithOperation(LoadSetOperation, "process"))
		require.NoError(t, err)

		assert.EqualValues(t, LoadSetOperation, opSet.operation.kind)
	})

	t.Run("withSpecs", func(t *testing.T) {
		opSet, err := NewOperationSet(WithSpecs(true))
		require.NoError(t, err)

		require.True(t, opSet.hasSpecs)
	})
}

// this suite is guarding against dotenv impl idiosyncrasies
func Test_OperationSet_Valueless(t *testing.T) {
	// interestingly dotenv impl return a value keyless
	t.Run("Naked spec parse valueless", func(t *testing.T) {
		naked := []string{"FOO"}

		opSet, err := NewOperationSet(WithOperation(LoadSetOperation, "naked"))
		require.NoError(t, err)

		err = opSet.addEnvs(naked...)
		require.NoError(t, err)

		require.Len(t, opSet.items, 1)
		require.EqualValues(t, "FOO", opSet.items["FOO"].Key)
		require.EqualValues(t, "", opSet.items["FOO"].Value.Resolved)
	})

	// interestingly dotenv impl return an empty map for standalone values
	t.Run("Naked specs parsed valueless", func(t *testing.T) {
		naked := []string{"BAR", "FOO", "BAZ"}

		opSet, err := NewOperationSet(WithOperation(LoadSetOperation, "naked"))
		require.NoError(t, err)

		err = opSet.addEnvs(naked...)
		require.NoError(t, err)

		require.Len(t, opSet.items, 3)
		require.EqualValues(t, "BAR", opSet.items["BAR"].Key)
		require.EqualValues(t, "", opSet.items["BAR"].Value.Resolved)

		require.EqualValues(t, "FOO", opSet.items["FOO"].Key)
		require.EqualValues(t, "", opSet.items["FOO"].Value.Resolved)

		require.EqualValues(t, "BAZ", opSet.items["BAZ"].Key)
		require.EqualValues(t, "", opSet.items["BAZ"].Value.Resolved)
	})
}

func Test_Store(t *testing.T) {
	t.Parallel()
	fake := []byte(`GOPATH=/Users/sourishkrout/go
INSTRUMENTATION_KEY=05a2cc58-5101-4c69-a0d0-7a126253a972 # Secret!
PGPASS=secret-fake-password # Password!
HOMEBREW_REPOSITORY=/opt/homebrew # Plain`)

	t.Run("Valildate query", func(t *testing.T) {
		store, err := NewStore(withSpecsFile(".env", fake, true))
		require.NoError(t, err)
		require.NotNil(t, store)

		var query, vars bytes.Buffer
		err = store.validateQuery(&query, &vars)
		require.NoError(t, err)

		// fmt.Println(query.String())
	})

	t.Run("Valildate specs", func(t *testing.T) {
		store, err := NewStore(withSpecsFile(".env.example", fake, true), WithEnvFile(".env", fake))
		// todo(sebastian): test the unresolved case (line below)
		// store, err := NewStore(withSpecsFile(".env", fake, true))
		require.NoError(t, err)
		require.NotNil(t, store)

		vars, err := store.snapshot(false)
		require.NoError(t, err)
		require.NotNil(t, vars)

		vars.sortbyKey()

		require.EqualValues(t, "GOPATH", vars[0].Key)
		require.EqualValues(t, "Opaque", vars[0].Spec.Name)
		require.EqualValues(t, "", vars[0].Value.Resolved)
		require.EqualValues(t, "/Users/sourishkrout/go", vars[0].Value.Original)
		require.EqualValues(t, "HIDDEN", vars[0].Value.Status)
		require.EqualValues(t, false, vars[0].Required)

		require.EqualValues(t, "HOMEBREW_REPOSITORY", vars[1].Key)
		require.EqualValues(t, "Plain", vars[1].Spec.Name)
		require.EqualValues(t, "/opt/homebrew", vars[1].Value.Resolved)
		require.EqualValues(t, "/opt/homebrew", vars[1].Value.Original)
		require.EqualValues(t, "LITERAL", vars[1].Value.Status)
		require.EqualValues(t, false, vars[1].Required)

		require.EqualValues(t, "INSTRUMENTATION_KEY", vars[2].Key)
		require.EqualValues(t, "Secret", vars[2].Spec.Name)
		require.EqualValues(t, "05a...972", vars[2].Value.Resolved)
		require.EqualValues(t, "", vars[2].Value.Original)
		require.EqualValues(t, "MASKED", vars[2].Value.Status)
		require.EqualValues(t, true, vars[2].Required)

		require.EqualValues(t, "PGPASS", vars[3].Key)
		require.EqualValues(t, "Password", vars[3].Spec.Name)
		require.EqualValues(t, "********************", vars[3].Value.Resolved)
		require.EqualValues(t, "", vars[3].Value.Original)
		require.EqualValues(t, "MASKED", vars[3].Value.Status)
		require.EqualValues(t, true, vars[3].Required)
	})

	t.Run("Validate with process envs", func(t *testing.T) {
		raw := []byte(`COMMAND_MODE=not-really-secret # Secret
INSTRUMENTATION_KEY=05a2cc58-5101-4c69-a0d0-7a126253a972 # Password!
HOME=fake-secret # Secret!
HOMEBREW_REPOSITORY=where homebrew lives # Plain`)
		envs := os.Environ()

		store, err := NewStore(WithEnvs(envs...), WithSpecFile(".env.example", raw))
		require.NoError(t, err)

		require.Len(t, store.opSets, 2)
		require.Len(t, store.opSets[0].items, len(envs))

		_, err = store.snapshot(true)
		require.NoError(t, err)

		// j, err := json.MarshalIndent(vars, "", " ")
		// require.NoError(t, err)

		// fmt.Println(string(j))
	})

	t.Run("Snapshot with empty env", func(t *testing.T) {
		raw := []byte(``)
		store, err := NewStore(WithSpecFile("empty", raw))
		require.NoError(t, err)

		require.Len(t, store.opSets, 1)
		require.Len(t, store.opSets[0].items, 0)

		snapshot, err := store.snapshot(false)
		require.NoError(t, err)
		require.Len(t, snapshot, 0)
	})

	// todo: this test-cases needs refactoring to run in CI
	// t.Run("Snapshot with fake env", func(t *testing.T) {
	// 	envs := os.Environ()

	// 	raw := []byte(`WASI_SDK_PATH=The path to the wasi-sdk directory # Plain!`)
	// 	store, err := NewStore(WithEnvs(envs...), WithSpecFile(".env.example", raw))
	// 	require.NoError(t, err)

	// 	require.Len(t, store.opSets, 2)
	// 	require.Len(t, store.opSets[0].items, len(envs))
	// 	require.Len(t, store.opSets[1].items, 1)

	// 	snapshot, err := store.snapshot(false)
	// 	require.NoError(t, err)
	// 	require.EqualValues(t, "/Users/sourishkrout/Projects/stateful/2022Q4/wasi-sdk/dist/wasi-sdk-16.5ga0a342ac182c", snapshot[0].Value.Resolved)
	// 	require.EqualValues(t, "", snapshot[0].Value.Original)
	// 	require.EqualValues(t, "Plain", snapshot[0].Spec.Name)
	// })

	t.Run("LoadEnv", func(t *testing.T) {
		// todo(sebastian): needs better solution
		rawEnvLocal, err := os.ReadFile("../../pkg/project/test_project/.env.local")
		require.NoError(t, err)
		rawEnv, err := os.ReadFile("../../pkg/project/test_project/.env")
		require.NoError(t, err)

		store, err := NewStore(
			// order matters
			WithEnvFile(".env.local", rawEnvLocal),
			WithEnvFile(".env", rawEnv),
		)
		require.NoError(t, err)

		require.Len(t, store.opSets, 2)
		require.Len(t, store.opSets[0].items, 2)
		require.Len(t, store.opSets[1].items, 2)

		snapshot, err := store.snapshot(true)
		require.NoError(t, err)
		require.Len(t, snapshot, 3)

		snapshot.sortbyKey()

		require.EqualValues(t, "secret1_overridden", snapshot[0].Value.Resolved)
		require.EqualValues(t, "secret1_overridden", snapshot[0].Value.Original)
		require.EqualValues(t, "LITERAL", snapshot[0].Value.Status)
		require.EqualValues(t, "Opaque", snapshot[0].Spec.Name)

		require.EqualValues(t, "secret2", snapshot[1].Value.Resolved)
		require.EqualValues(t, "secret2", snapshot[1].Value.Original)
		require.EqualValues(t, "LITERAL", snapshot[1].Value.Status)
		require.EqualValues(t, "Opaque", snapshot[1].Spec.Name)

		require.EqualValues(t, "secret3", snapshot[2].Value.Resolved)
		require.EqualValues(t, "secret3", snapshot[2].Value.Original)
		require.EqualValues(t, "LITERAL", snapshot[2].Value.Status)
		require.EqualValues(t, "Opaque", snapshot[2].Spec.Name)
	})
}

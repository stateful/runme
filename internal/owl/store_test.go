//go:build !windows

package owl

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_OperationSet(t *testing.T) {
	t.Parallel()

	t.Run("withOperation", func(t *testing.T) {
		opSet, err := NewOperationSet(WithOperation(LoadSetOperation))
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

		opSet, err := NewOperationSet(WithOperation(LoadSetOperation))
		require.NoError(t, err)

		err = opSet.addEnvs("naked", naked...)
		require.NoError(t, err)

		require.Len(t, opSet.values, 1)
		require.EqualValues(t, "FOO", opSet.values["FOO"].Var.Key)
		require.EqualValues(t, "", opSet.values["FOO"].Value.Resolved)
	})

	// interestingly dotenv impl return an empty map for standalone values
	t.Run("Naked specs parsed valueless", func(t *testing.T) {
		naked := []string{"BAR", "FOO", "BAZ"}

		opSet, err := NewOperationSet(WithOperation(LoadSetOperation))
		require.NoError(t, err)

		err = opSet.addEnvs("naked", naked...)
		require.NoError(t, err)

		require.Len(t, opSet.values, 3)
		require.EqualValues(t, "BAR", opSet.values["BAR"].Var.Key)
		require.EqualValues(t, "", opSet.values["BAR"].Value.Resolved)

		require.EqualValues(t, "FOO", opSet.values["FOO"].Var.Key)
		require.EqualValues(t, "", opSet.values["FOO"].Value.Resolved)

		require.EqualValues(t, "BAZ", opSet.values["BAZ"].Var.Key)
		require.EqualValues(t, "", opSet.values["BAZ"].Value.Resolved)
	})
}

var fake = []byte(`GOPATH=/Users/sourishkrout/go
INSTRUMENTATION_KEY=05a2cc58-5101-4c69-a0d0-7a126253a972 # Secret!
PGPASS=secret-fake-password # Password!
HOMEBREW_REPOSITORY=/opt/homebrew # Plain`)

func Test_Store(t *testing.T) {
	t.Parallel()

	t.Run("Sensitive query", func(t *testing.T) {
		store, err := NewStore(withSpecsFile(".env", fake, false))
		require.NoError(t, err)
		require.NotNil(t, store)

		var query, vars bytes.Buffer
		err = store.sensitiveKeysQuery(&query, &vars)
		require.NoError(t, err)

		fmt.Println(query.String())
	})

	t.Run("Validate with process envs", func(t *testing.T) {
		raw := []byte(`COMMAND_MODE=not-really-secret # Secret
INSTRUMENTATION_KEY=05a2cc58-5101-4c69-a0d0-7a126253a972 # Password!
HOME=fake-secret # Secret!
HOMEBREW_REPOSITORY=where homebrew lives # Plain`)
		envs := os.Environ()

		store, err := NewStore(WithEnvs("[system]", envs...), WithSpecFile(".env.example", raw))
		require.NoError(t, err)

		require.Len(t, store.opSets, 2)
		require.Len(t, store.opSets[0].values, len(envs))

		snapshot, err := store.snapshot(true)
		require.NoError(t, err)

		require.Greater(t, len(snapshot), 4)

		// j, err := json.MarshalIndent(snapshot, "", " ")
		// require.NoError(t, err)

		// fmt.Println(string(j))
	})

	t.Run("Snapshot with empty env", func(t *testing.T) {
		raw := []byte(``)
		store, err := NewStore(WithSpecFile("empty", raw))
		require.NoError(t, err)

		require.Len(t, store.opSets, 1)
		require.Len(t, store.opSets[0].values, 0)

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
	// 	require.Len(t, store.opSets[0].values, len(envs))
	// 	require.Len(t, store.opSets[1].values, 1)

	// 	snapshot, err := store.snapshot(false)
	// 	require.NoError(t, err)
	// 	require.EqualValues(t, "/Users/sourishkrout/Projects/stateful/2022Q4/wasi-sdk/dist/wasi-sdk-16.5ga0a342ac182c", snapshot[0].Value.Resolved)
	// 	require.EqualValues(t, "", snapshot[0].Value.Original)
	// 	require.EqualValues(t, "Plain", snapshot[0].Spec.Name)
	// })
}

func Test_Store_Specless(t *testing.T) {
	t.Parallel()

	rawEnvLocal, err := os.ReadFile("testdata/project/.env.local")
	require.NoError(t, err)
	rawEnv, err := os.ReadFile("testdata/project/.env")
	require.NoError(t, err)

	store, err := NewStore(
		// order matters
		WithEnvFile(".env.local", rawEnvLocal),
		WithEnvFile(".env", rawEnv),
	)
	require.NoError(t, err)

	require.Len(t, store.opSets, 2)
	require.Len(t, store.opSets[0].values, 2)
	require.Len(t, store.opSets[1].values, 2)

	t.Run("with insecure true", func(t *testing.T) {
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

	t.Run("with insecure false", func(t *testing.T) {
		snapshot, err := store.snapshot(false)
		require.NoError(t, err)
		require.Len(t, snapshot, 3)

		snapshot.sortbyKey()

		require.EqualValues(t, "", snapshot[0].Value.Resolved)
		require.EqualValues(t, "secret1_overridden", snapshot[0].Value.Original)
		require.EqualValues(t, "HIDDEN", snapshot[0].Value.Status)
		require.EqualValues(t, "Opaque", snapshot[0].Spec.Name)

		require.EqualValues(t, "", snapshot[1].Value.Resolved)
		require.EqualValues(t, "secret2", snapshot[1].Value.Original)
		require.EqualValues(t, "HIDDEN", snapshot[1].Value.Status)
		require.EqualValues(t, "Opaque", snapshot[1].Spec.Name)

		require.EqualValues(t, "", snapshot[2].Value.Resolved)
		require.EqualValues(t, "secret3", snapshot[2].Value.Original)
		require.EqualValues(t, "HIDDEN", snapshot[2].Value.Status)
		require.EqualValues(t, "Opaque", snapshot[2].Spec.Name)
	})
}

func Test_Store_FixtureWithSpecs(t *testing.T) {
	t.Parallel()

	store, err := NewStore(withSpecsFile(".env.example", fake, true), WithEnvFile(".env", fake))
	require.NoError(t, err)
	require.NotNil(t, store)

	t.Run("Insecure is false", func(t *testing.T) {
		snapshot, err := store.snapshot(false)
		require.NoError(t, err)
		require.NotNil(t, snapshot)

		snapshot.sortbyKey()

		require.EqualValues(t, "GOPATH", snapshot[0].Var.Key)
		require.EqualValues(t, "Opaque", snapshot[0].Spec.Name)
		require.EqualValues(t, false, snapshot[0].Spec.Required)
		require.EqualValues(t, "", snapshot[0].Value.Resolved)
		require.EqualValues(t, "/Users/sourishkrout/go", snapshot[0].Value.Original)
		require.EqualValues(t, "HIDDEN", snapshot[0].Value.Status)

		require.EqualValues(t, "HOMEBREW_REPOSITORY", snapshot[1].Var.Key)
		require.EqualValues(t, "Plain", snapshot[1].Spec.Name)
		require.EqualValues(t, false, snapshot[1].Spec.Required)
		require.EqualValues(t, "/opt/homebrew", snapshot[1].Value.Resolved)
		require.EqualValues(t, "/opt/homebrew", snapshot[1].Value.Original)
		require.EqualValues(t, "LITERAL", snapshot[1].Value.Status)

		require.EqualValues(t, "INSTRUMENTATION_KEY", snapshot[2].Var.Key)
		require.EqualValues(t, "Secret", snapshot[2].Spec.Name)
		require.EqualValues(t, true, snapshot[2].Spec.Required)
		require.EqualValues(t, "05a...972", snapshot[2].Value.Resolved)
		require.EqualValues(t, "", snapshot[2].Value.Original)
		require.EqualValues(t, "MASKED", snapshot[2].Value.Status)

		require.EqualValues(t, "PGPASS", snapshot[3].Var.Key)
		require.EqualValues(t, "Password", snapshot[3].Spec.Name)
		require.EqualValues(t, true, snapshot[3].Spec.Required)
		require.EqualValues(t, "********************", snapshot[3].Value.Resolved)
		require.EqualValues(t, "", snapshot[3].Value.Original)
		require.EqualValues(t, "MASKED", snapshot[3].Value.Status)
	})

	t.Run("Insecure is true", func(t *testing.T) {
		snapshot, err := store.snapshot(true)
		require.NoError(t, err)
		require.NotNil(t, snapshot)

		snapshot.sortbyKey()

		require.EqualValues(t, "GOPATH", snapshot[0].Var.Key)
		require.EqualValues(t, "Opaque", snapshot[0].Spec.Name)
		require.EqualValues(t, false, snapshot[0].Spec.Required)
		require.EqualValues(t, "/Users/sourishkrout/go", snapshot[0].Value.Resolved)
		require.EqualValues(t, "/Users/sourishkrout/go", snapshot[0].Value.Original)
		require.EqualValues(t, "LITERAL", snapshot[0].Value.Status)

		require.EqualValues(t, "HOMEBREW_REPOSITORY", snapshot[1].Var.Key)
		require.EqualValues(t, "Plain", snapshot[1].Spec.Name)
		require.EqualValues(t, false, snapshot[1].Spec.Required)
		require.EqualValues(t, "/opt/homebrew", snapshot[1].Value.Resolved)
		require.EqualValues(t, "/opt/homebrew", snapshot[1].Value.Original)
		require.EqualValues(t, "LITERAL", snapshot[1].Value.Status)

		require.EqualValues(t, "INSTRUMENTATION_KEY", snapshot[2].Var.Key)
		require.EqualValues(t, "Secret", snapshot[2].Spec.Name)
		require.EqualValues(t, true, snapshot[2].Spec.Required)
		require.EqualValues(t, "05a2cc58-5101-4c69-a0d0-7a126253a972", snapshot[2].Value.Resolved)
		require.EqualValues(t, "05a2cc58-5101-4c69-a0d0-7a126253a972", snapshot[2].Value.Original)
		require.EqualValues(t, "LITERAL", snapshot[2].Value.Status)

		require.EqualValues(t, "PGPASS", snapshot[3].Var.Key)
		require.EqualValues(t, "Password", snapshot[3].Spec.Name)
		require.EqualValues(t, true, snapshot[3].Spec.Required)
		require.EqualValues(t, "secret-fake-password", snapshot[3].Value.Resolved)
		require.EqualValues(t, "secret-fake-password", snapshot[3].Value.Original)
		require.EqualValues(t, "LITERAL", snapshot[3].Value.Status)
	})
}

func Test_Store_FixtureWithoutSpecs(t *testing.T) {
	t.Parallel()

	store, err := NewStore(WithEnvFile(".env", fake))
	require.NoError(t, err)
	require.NotNil(t, store)

	t.Run("Insecure is false", func(t *testing.T) {
		snapshot, err := store.snapshot(false)
		require.NoError(t, err)
		require.NotNil(t, snapshot)

		snapshot.sortbyKey()

		require.EqualValues(t, "GOPATH", snapshot[0].Var.Key)
		require.EqualValues(t, "Opaque", snapshot[0].Spec.Name)
		require.EqualValues(t, false, snapshot[0].Spec.Required)
		require.EqualValues(t, "", snapshot[0].Value.Resolved)
		require.EqualValues(t, "/Users/sourishkrout/go", snapshot[0].Value.Original)
		require.EqualValues(t, "HIDDEN", snapshot[0].Value.Status)

		require.EqualValues(t, "HOMEBREW_REPOSITORY", snapshot[1].Var.Key)
		require.EqualValues(t, "Opaque", snapshot[1].Spec.Name)
		require.EqualValues(t, false, snapshot[1].Spec.Required)
		require.EqualValues(t, "", snapshot[1].Value.Resolved)
		require.EqualValues(t, "/opt/homebrew", snapshot[1].Value.Original)
		require.EqualValues(t, "HIDDEN", snapshot[1].Value.Status)

		require.EqualValues(t, "INSTRUMENTATION_KEY", snapshot[2].Var.Key)
		require.EqualValues(t, "Opaque", snapshot[2].Spec.Name)
		require.EqualValues(t, false, snapshot[2].Spec.Required)
		require.EqualValues(t, "", snapshot[2].Value.Resolved)
		require.EqualValues(t, "05a2cc58-5101-4c69-a0d0-7a126253a972", snapshot[2].Value.Original)
		require.EqualValues(t, "HIDDEN", snapshot[2].Value.Status)

		require.EqualValues(t, "PGPASS", snapshot[3].Var.Key)
		require.EqualValues(t, "Opaque", snapshot[3].Spec.Name)
		require.EqualValues(t, false, snapshot[3].Spec.Required)
		require.EqualValues(t, "", snapshot[3].Value.Resolved)
		require.EqualValues(t, "secret-fake-password", snapshot[3].Value.Original)
		require.EqualValues(t, "HIDDEN", snapshot[3].Value.Status)
	})

	t.Run("Insecure is true", func(t *testing.T) {
		snapshot, err := store.snapshot(true)
		require.NoError(t, err)
		require.NotNil(t, snapshot)

		snapshot.sortbyKey()

		require.EqualValues(t, "GOPATH", snapshot[0].Var.Key)
		require.EqualValues(t, "Opaque", snapshot[0].Spec.Name)
		require.EqualValues(t, false, snapshot[0].Spec.Required)
		require.EqualValues(t, "/Users/sourishkrout/go", snapshot[0].Value.Resolved)
		require.EqualValues(t, "/Users/sourishkrout/go", snapshot[0].Value.Original)
		require.EqualValues(t, "LITERAL", snapshot[0].Value.Status)

		require.EqualValues(t, "HOMEBREW_REPOSITORY", snapshot[1].Var.Key)
		require.EqualValues(t, "Opaque", snapshot[1].Spec.Name)
		require.EqualValues(t, false, snapshot[1].Spec.Required)
		require.EqualValues(t, "/opt/homebrew", snapshot[1].Value.Resolved)
		require.EqualValues(t, "/opt/homebrew", snapshot[1].Value.Original)
		require.EqualValues(t, "LITERAL", snapshot[1].Value.Status)

		require.EqualValues(t, "INSTRUMENTATION_KEY", snapshot[2].Var.Key)
		require.EqualValues(t, "Opaque", snapshot[2].Spec.Name)
		require.EqualValues(t, false, snapshot[2].Spec.Required)
		require.EqualValues(t, "05a2cc58-5101-4c69-a0d0-7a126253a972", snapshot[2].Value.Resolved)
		require.EqualValues(t, "05a2cc58-5101-4c69-a0d0-7a126253a972", snapshot[2].Value.Original)
		require.EqualValues(t, "LITERAL", snapshot[2].Value.Status)

		require.EqualValues(t, "PGPASS", snapshot[3].Var.Key)
		require.EqualValues(t, "Opaque", snapshot[3].Spec.Name)
		require.EqualValues(t, false, snapshot[3].Spec.Required)
		require.EqualValues(t, "secret-fake-password", snapshot[3].Value.Resolved)
		require.EqualValues(t, "secret-fake-password", snapshot[3].Value.Original)
		require.EqualValues(t, "LITERAL", snapshot[3].Value.Status)
	})
}

func Test_Store_Validation(t *testing.T) {
	t.Parallel()

	fakeErrs := []byte(`GOPATH=
INSTRUMENTATION_KEY=Instrumentation key for env # Secret!
PGPASS=Your database password # Password!
HOMEBREW_REPOSITORY= # Plain`)
	store, err := NewStore(WithSpecFile(".env.example", fakeErrs))
	require.NoError(t, err)
	require.NotNil(t, store)

	t.Run("Insecure is false", func(t *testing.T) {
		snapshot, err := store.snapshot(false)
		require.NoError(t, err)
		require.NotNil(t, snapshot)

		snapshot.sortbyKey()

		snapshot0 := snapshot[0]
		require.EqualValues(t, "GOPATH", snapshot0.Var.Key)
		require.EqualValues(t, "Opaque", snapshot0.Spec.Name)
		require.EqualValues(t, false, snapshot0.Spec.Required)
		require.EqualValues(t, "", snapshot0.Value.Resolved)
		require.EqualValues(t, "", snapshot0.Value.Original)
		require.EqualValues(t, "UNRESOLVED", snapshot0.Value.Status)
		require.LessOrEqual(t, len(snapshot0.Errors), 0)

		snapshot1 := snapshot[1]
		require.EqualValues(t, "HOMEBREW_REPOSITORY", snapshot1.Var.Key)
		require.EqualValues(t, "Plain", snapshot1.Spec.Name)
		require.EqualValues(t, false, snapshot1.Spec.Required)
		require.EqualValues(t, "", snapshot1.Value.Resolved)
		require.EqualValues(t, "", snapshot1.Value.Original)
		require.EqualValues(t, "UNRESOLVED", snapshot1.Value.Status)
		require.LessOrEqual(t, len(snapshot1.Errors), 0)

		snapshot2 := snapshot[2]
		require.EqualValues(t, "INSTRUMENTATION_KEY", snapshot2.Var.Key)
		require.EqualValues(t, "Secret", snapshot2.Spec.Name)
		require.EqualValues(t, true, snapshot2.Spec.Required)
		require.EqualValues(t, "", snapshot2.Value.Resolved)
		require.EqualValues(t, "", snapshot2.Value.Original)
		require.EqualValues(t, "UNRESOLVED", snapshot2.Value.Status)
		require.Greater(t, len(snapshot2.Errors), 0)
		require.EqualValues(t, snapshot2.Errors[0].Code, 0)
		require.EqualValues(t, snapshot2.Errors[0], &SetVarError{Code: 0, Message: "Error 0: Variable \"INSTRUMENTATION_KEY\" is unresolved but declared as required by \"Secret!\" in \".env.example\""})

		snapshot3 := snapshot[3]
		require.EqualValues(t, "PGPASS", snapshot3.Var.Key)
		require.EqualValues(t, "Password", snapshot3.Spec.Name)
		require.EqualValues(t, true, snapshot3.Spec.Required)
		require.EqualValues(t, "", snapshot3.Value.Resolved)
		require.EqualValues(t, "", snapshot3.Value.Original)
		require.EqualValues(t, "UNRESOLVED", snapshot3.Value.Status)
		require.Greater(t, len(snapshot3.Errors), 0)
		require.EqualValues(t, snapshot3.Errors[0], &SetVarError{Code: 0, Message: "Error 0: Variable \"PGPASS\" is unresolved but declared as required by \"Password!\" in \".env.example\""})
	})
}

func Test_Store_Reconcile(t *testing.T) {
	t.Parallel()
	fake := []byte(`UNRESOLVED_SECRET_WITHOUT_VALUE= # Secret`)

	t.Run("exclude unresolved values from insecure snapshot", func(t *testing.T) {
		store, err := NewStore(withSpecsFile(".env.example", fake, true))
		require.NoError(t, err)
		require.NotNil(t, store)

		snapshot, err := store.snapshot(true)
		require.NoError(t, err)

		require.Equal(t, 0, len(snapshot))
	})

	t.Run("include unresolved values from secure snapshot", func(t *testing.T) {
		store, err := NewStore(withSpecsFile(".env.example", fake, true))
		require.NoError(t, err)
		require.NotNil(t, store)

		snapshot, err := store.snapshot(false)
		require.NoError(t, err)
		require.Equal(t, 1, len(snapshot))

		snapshot0 := snapshot[0]

		require.EqualValues(t, "UNRESOLVED_SECRET_WITHOUT_VALUE", snapshot0.Var.Key)
		require.EqualValues(t, "Secret", snapshot0.Spec.Name)
		require.EqualValues(t, false, snapshot0.Spec.Required)
		require.EqualValues(t, "", snapshot0.Value.Resolved)
		require.EqualValues(t, "", snapshot0.Value.Original)
		require.EqualValues(t, "UNRESOLVED", snapshot0.Value.Status)
		require.LessOrEqual(t, len(snapshot0.Errors), 0)
	})
}

func Test_Store_SensitiveKeys(t *testing.T) {
	t.Parallel()

	store, err := NewStore(withSpecsFile(".env.example", fake, true), WithEnvFile(".env", fake))
	require.NoError(t, err)
	require.NotNil(t, store)

	keys, err := store.SensitiveKeys()
	require.NoError(t, err)
	require.EqualValues(t, keys, []string{"INSTRUMENTATION_KEY", "PGPASS"})
}

func Test_Store_SecretMasking(t *testing.T) {
	t.Parallel()

	t.Run("Short secret is masked as empty", func(t *testing.T) {
		fake := []byte("SHORT_SECRET=extra-short # Secret!\n")

		store, err := NewStore(withSpecsFile(".env.example", fake, true), WithEnvFile(".env", fake))
		require.NoError(t, err)
		require.NotNil(t, store)

		snapshot, err := store.snapshot(false)
		require.NoError(t, err)
		require.NotNil(t, snapshot)

		snapshot.sortbyKey()

		snapshot0 := snapshot[0]
		require.EqualValues(t, "SHORT_SECRET", snapshot0.Var.Key)
		require.EqualValues(t, "Secret", snapshot0.Spec.Name)
		require.EqualValues(t, true, snapshot0.Spec.Required)
		require.EqualValues(t, "", snapshot0.Value.Resolved)
		require.EqualValues(t, "", snapshot0.Value.Original)
		require.EqualValues(t, "MASKED", snapshot0.Value.Status)
		require.LessOrEqual(t, len(snapshot0.Errors), 0)
	})

	t.Run("Long secret greater than 24 chars shows glimpse", func(t *testing.T) {
		fake := []byte("LONG_SECRET=this-is-a-extra-long-secret-which-is-much-better-practice # Secret!\n")

		store, err := NewStore(withSpecsFile(".env.example", fake, true), WithEnvFile(".env", fake))
		require.NoError(t, err)
		require.NotNil(t, store)

		snapshot, err := store.snapshot(false)
		require.NoError(t, err)
		require.NotNil(t, snapshot)

		snapshot.sortbyKey()

		snapshot0 := snapshot[0]
		require.EqualValues(t, "LONG_SECRET", snapshot0.Var.Key)
		require.EqualValues(t, "Secret", snapshot0.Spec.Name)
		require.EqualValues(t, true, snapshot0.Spec.Required)
		require.EqualValues(t, `thi...ice`, snapshot0.Value.Resolved)
		require.EqualValues(t, "", snapshot0.Value.Original)
		require.EqualValues(t, "MASKED", snapshot0.Value.Status)
		require.LessOrEqual(t, len(snapshot0.Errors), 0)
	})
}

func Test_Store_Get(t *testing.T) {
	store, err := NewStore(withSpecsFile(".env.example", fake, true), WithEnvFile(".env", fake))
	require.NoError(t, err)
	require.NotNil(t, store)

	// PGPASS is masked without insecure
	val, err := store.InsecureGet("PGPASS")
	require.NoError(t, err)
	assert.EqualValues(t, "secret-fake-password", val)
}

//go:build !windows

package owl

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Store_ComplexSpecs(t *testing.T) {
	fakeNS := []byte(`GOPATH=/Users/sourishkrout/go
	INSTRUMENTATION_KEY=05a2cc58-5101-4c69-a0d0-7a126253a972 # Secret!
	PGPASS=secret-fake-password # Password!
	HOMEBREW_REPOSITORY=/opt/homebrew # Plain
	POSTGRES_HOST=127.0.0.1 # Postgres!
	QUEUES_REDIS_HOST=127.0.0.2 # Redis!
	QUEUES_REDIS_PORT=6379 # Redis!
	PUBSUB_REDIS_HOST=127.0.0.3 # Redis!
	PUBSUB_REDIS_PORT=6379 # Redis!
	RATELIMITER_REDIS_HOST=127.0.0.4 # Redis
	GCLOUD_2_REDIS_HOST=127.0.0.5 # Redis
	REDIS_PASSWORD=fake-redis-password # Redis!
	REDIS_HOST=localhost # Redis!
	REDIS_PORT=6379 # Redis!`)

	t.Run("Getter without namespace", func(t *testing.T) {
		fake := []byte(`GOPATH=/Users/sourishkrout/go
		INSTRUMENTATION_KEY=05a2cc58-5101-4c69-a0d0-7a126253a972 # Secret!
		PGPASS=secret-fake-password # Password!
		HOMEBREW_REPOSITORY=/opt/homebrew # Plain
		REDIS_HOST=localhost # Redis!
		REDIS_PORT=6379 # Redis!`)

		store, err := NewStore(withSpecsFile(".env.example", fake, true), WithEnvFile(".env", fake))
		require.NoError(t, err)
		require.NotNil(t, store)

		val, err := store.InsecureGet("REDIS_HOST")
		require.NoError(t, err)
		assert.EqualValues(t, "localhost", val)
	})

	t.Run("Getter with namespaces", func(t *testing.T) {
		store, err := NewStore(withSpecsFile(".env.example", fakeNS, true), WithEnvFile(".env", fakeNS))
		require.NoError(t, err)
		require.NotNil(t, store)

		val, err := store.InsecureGet("REDIS_HOST")
		require.NoError(t, err)
		assert.EqualValues(t, "localhost", val)
	})

	t.Run("Snapshot with namespaces", func(t *testing.T) {
		store, err := NewStore(withSpecsFile(".env.example", fakeNS, true), WithEnvFile(".env", fakeNS))
		require.NoError(t, err)
		require.NotNil(t, store)

		snapshot, err := store.Snapshot()
		require.NoError(t, err)

		snapshot.sortbyKey()
		assert.EqualValues(t, "GCLOUD_2_REDIS_HOST", snapshot[0].Var.Key)
		assert.EqualValues(t, "127.0.0.5", snapshot[0].Value.Resolved)
	})

	t.Run("Validation errors for invalid env values", func(t *testing.T) {
		fake := []byte(`GOPATH=/Users/sourishkrout/go
	INSTRUMENTATION_KEY=05a2cc58-5101-4c69-a0d0-7a126253a972 # Secret!
	PGPASS=too-short # Password!
	HOMEBREW_REPOSITORY=/opt/homebrew # Plain
	REDIS_HOST=12345 # Redis!
	REDIS_PORT=invalid-port # Redis!`)
		store, err := NewStore(withSpecsFile(".env.example", fake, true), WithEnvFile(".env", fake))
		require.NoError(t, err)
		require.NotNil(t, store)

		snapshot, err := store.Snapshot()
		require.NoError(t, err)

		snapshot.sortbyKey()
		assert.EqualValues(t, "REDIS_HOST", snapshot[4].Var.Key)
		assert.EqualValues(t, "12345", snapshot[4].Value.Resolved)
		assert.EqualValues(t,
			`Error 1: The value of variable "REDIS_HOST" failed tag validation "ip|hostname" required by "Redis!" declared in ".env.example"`,
			snapshot[4].Errors[0].Message,
		)

		assert.EqualValues(t, "REDIS_PORT", snapshot[5].Var.Key)
		assert.EqualValues(t, "invalid-port", snapshot[5].Value.Resolved)
		assert.EqualValues(t,
			`Error 1: The value of variable "REDIS_PORT" failed tag validation "number" required by "Redis!" declared in ".env.example"`,
			snapshot[5].Errors[0].Message,
		)
	})
}

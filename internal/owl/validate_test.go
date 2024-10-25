//go:build !windows

package owl

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_Fixture(t *testing.T) {
	specs, err := os.ReadFile("testdata/validate/.env.example")
	require.NoError(t, err)
	values, err := os.ReadFile("testdata/validate/.env")
	require.NoError(t, err)

	store, err := NewStore(withSpecsFile(".env.example", specs, true), WithEnvFile(".env", values))
	require.NoError(t, err)

	snapshot, err := store.Snapshot()
	require.NoError(t, err)

	for _, item := range snapshot {
		assert.EqualValues(t, []*SetVarError{}, item.Errors)
	}
}

// func TestStore_JWTValidator(t *testing.T) {
// 	t.Run("Unexpired", func(t *testing.T) {
// 		expired := []byte(`AUTH_TOKEN=eyJhbGciOiJIUzUxMiIsInR5cCI6IkpXVCJ9.eyJhdWQiOlsiaHR0cHM6Ly91cy1jZW50cmFsMS5zdGF0ZWZ1bC5jb20vIiwiaHR0cHM6Ly9zdGF0ZWZ1bC1pbmMudXMuYXV0aDAuY29tL3VzZXJpbmZvIl0sImF6cCI6ImdvLXRlc3QiLCJleHAiOjI3MjI1NDg3MDEsImh0dHBzOi8vdXMtY2VudHJhbDEuc3RhdGVmdWwuY29tL2FwcF9tZXRhZGF0YSI6eyJ1c2VySWQiOiI0M2ZlNjc5Yy0zYTdmLTQ0NjUtYTU2Ni03NzEyMGQzOTA2MDcifSwiaWF0IjoxNzIyNTQ4NzAxLCJpc3MiOiJodHRwczovL2lkZW50aXR5LnN0YXRlZnVsLmNvbS8iLCJqdGkiOiJiZDI3ZmJhZTkwMjlmMWQwYTllMGM4M2Y0ZTkzMTZjYTAxZTM5Zjc5YTI5OWFkYTIzMDE2MTg2ZmIzMDg0NzhhIiwibmJmIjoxNzI5NjExNTYwLCJwZXJtaXNzaW9ucyI6W10sInNjb3BlIjoib3BlbmlkIHByb2ZpbGUgZW1haWwiLCJzdWIiOiJnb29nbGUtZmF1eC1vYXV0aDJ8OTkxOTQ0NzYxOTQ3OTAxMiJ9.GZXG1VHv87Qf5qk2di3gq7zJqqzY0wscdrQ0o9j86ZGf1w3_Tckb57XrrrkwGmgNSBAvPqf0SJNtGiXA41lZ8w # UnexpiredJWT!`)

// 		store, err := NewStore(withSpecsFile(".env.example", expired, true), WithEnvFile(".env", expired))
// 		require.NoError(t, err)
// 		require.NotNil(t, store)

// 		snapshot, err := store.Snapshot()
// 		require.NoError(t, err)

// 		assert.EqualValues(t, []*SetVarError{}, snapshot[0].Errors)
// 	})
// 	t.Run("Expired", func(t *testing.T) {
// 		expired := []byte(`AUTH_TOKEN=eyJhbGciOiJIUzUxMiIsInR5cCI6IkpXVCJ9.eyJhdWQiOlsiaHR0cHM6Ly91cy1jZW50cmFsMS5zdGF0ZWZ1bC5jb20vIiwiaHR0cHM6Ly9zdGF0ZWZ1bC1pbmMudXMuYXV0aDAuY29tL3VzZXJpbmZvIl0sImF6cCI6ImdvLXRlc3QiLCJleHAiOjE3MDI1NDg3MDEsImh0dHBzOi8vdXMtY2VudHJhbDEuc3RhdGVmdWwuY29tL2FwcF9tZXRhZGF0YSI6eyJ1c2VySWQiOiI0M2ZlNjc5Yy0zYTdmLTQ0NjUtYTU2Ni03NzEyMGQzOTA2MDcifSwiaWF0IjoxNzIyNTQ4NzAxLCJpc3MiOiJodHRwczovL2lkZW50aXR5LnN0YXRlZnVsLmNvbS8iLCJqdGkiOiI2ZTAxYzM1NGJlOTY2ZjZlYjI5ZTAyOWQ3ZjczMzIzYWE2N2VmNzViNThhOGQwOTY0YzBhYTM5NDRlYTY2YWZlIiwibmJmIjoxNzI5NjExNjUzLCJwZXJtaXNzaW9ucyI6W10sInNjb3BlIjoib3BlbmlkIHByb2ZpbGUgZW1haWwiLCJzdWIiOiJnb29nbGUtZmF1eC1vYXV0aDJ8OTkxOTQ0NzYxOTQ3OTAxMiJ9.14g3Aeni-pZJZOihP5JeSuyVHLKHG4lA2bI1wMl-XgexFZdSE7VTbCGiUPSJu4dvU0z603u4fxioZ3TdEMvIzQ # UnexpiredJWT!`)

// 		store, err := NewStore(withSpecsFile(".env.example", expired, true), WithEnvFile(".env", expired))
// 		require.NoError(t, err)
// 		require.NotNil(t, store)

// 		snapshot, err := store.Snapshot()
// 		require.NoError(t, err)

// 		assert.EqualValues(t, []*SetVarError{{Code: int(ValidateErrorJwtFailed), Message: `Error 3: The value of variable "AUTH_TOKEN" failed JWT validation (exp claim expired) required by "UnexpiredJWT->TOKEN" declared in ".env.example"`}}, snapshot[0].Errors)
// 	})

// 	t.Run("Malformed", func(t *testing.T) {
// 		expired := []byte(`AUTH_TOKEN=this.is-not.a-jwt # UnexpiredJWT!`)

// 		store, err := NewStore(withSpecsFile(".env.example", expired, true), WithEnvFile(".env", expired))
// 		require.NoError(t, err)
// 		require.NotNil(t, store)

// 		snapshot, err := store.Snapshot()
// 		require.NoError(t, err)

// 		assert.EqualValues(t, []*SetVarError{{Code: int(ValidateErrorJwtFailed), Message: `Error 3: The value of variable "AUTH_TOKEN" failed JWT validation (illegal base64 data at input byte 4) required by "UnexpiredJWT->TOKEN" declared in ".env.example"`}}, snapshot[0].Errors)
// 	})
// }

func TestStore_Specs(t *testing.T) {
	t.Run("Auth0", func(t *testing.T) {
		fake := []byte(`AUTH0_DOMAIN=stateful-staging.us.auth0.com # Auth0
			AUTH0_CLIENT_ID=WVKBKL2b8asAb8e3gRGDK0tGlTGQjkEV # Auth0
			AUTH0_AUDIENCE=https://staging.us-central1.stateful.com/ # Auth0`)

		store, err := NewStore(withSpecsFile(".env.example", fake, true), WithEnvFile(".env", fake))
		require.NoError(t, err)
		require.NotNil(t, store)

		snapshot, err := store.Snapshot()
		require.NoError(t, err)

		require.EqualValues(t, "https://staging.us-central1.stateful.com/", snapshot[0].Value.Resolved)
		require.EqualValues(t, "WVKBKL2b8asAb8e3gRGDK0tGlTGQjkEV", snapshot[1].Value.Resolved)
		require.EqualValues(t, "stateful-staging.us.auth0.com", snapshot[2].Value.Resolved)

		for _, item := range snapshot {
			assert.EqualValues(t, []*SetVarError{}, item.Errors)
		}
	})

	t.Run("Auth0Mgmt", func(t *testing.T) {
		fake := []byte(`AUTH0_MANAGEMENT_CLIENT_ID=8Esb35AdZi3eLOy8WIkHTrsFVACu43aB # Auth0Mgmt
			AUTH0_MANAGEMENT_CLIENT_SECRET=sxrt0A_3bs-7xfakeRFfakecuuGZfakeTQOfakeg-jBFmm_fake447fake30HI5o # Auth0Mgmt
			AUTH0_MANAGEMENT_AUDIENCE=https://stateful-staging.us.auth0.com/api/v2/ # Auth0Mgmt`)

		store, err := NewStore(withSpecsFile(".env.example", fake, true), WithEnvFile(".env", fake))
		require.NoError(t, err)
		require.NotNil(t, store)

		snapshot, err := store.Snapshot()
		require.NoError(t, err)

		require.EqualValues(t, "https://stateful-staging.us.auth0.com/api/v2/", snapshot[0].Value.Resolved)
		require.EqualValues(t, "8Esb35AdZi3eLOy8WIkHTrsFVACu43aB", snapshot[1].Value.Resolved)
		require.EqualValues(t, "sxrt0A_3bs-7xfakeRFfakecuuGZfakeTQOfakeg-jBFmm_fake447fake30HI5o", snapshot[2].Value.Resolved)

		for _, item := range snapshot {
			assert.EqualValues(t, []*SetVarError{}, item.Errors)
		}
	})

	t.Run("OpenAI", func(t *testing.T) {
		fake := []byte(`OPENAI_ORG_ID=org-tmfakeynfake9fakeHfakek0 # OpenAI
			OPENAI_API_KEY=sk-proj-HfakeVID3D_fakeThBgfake84BWfakeEcfakeMvfakeBZfake3fakeJ--qfakeLpfakeHgfakeT3fakeFfake2ZfakevfakerfakeJfakedfake0fakeEfakeofakeyjnNfaketyfakeSyfakeGvfakefake # OpenAI`)

		store, err := NewStore(withSpecsFile(".env.example", fake, true), WithEnvFile(".env", fake))
		require.NoError(t, err)
		require.NotNil(t, store)

		snapshot, err := store.Snapshot()
		require.NoError(t, err)

		require.EqualValues(t, "sk-...ake", snapshot[0].Value.Resolved)
		require.EqualValues(t, "org-tmfakeynfake9fakeHfakek0", snapshot[1].Value.Resolved)

		for _, item := range snapshot {
			assert.EqualValues(t, []*SetVarError{}, item.Errors)
		}
	})
}

func TestStore_ComplexSpecs(t *testing.T) {
	fakeNS := []byte(`GOPATH=/Users/sourishkrout/go
	INSTRUMENTATION_KEY=05a2cc58-5101-4c69-a0d0-7a126253a972 # Secret!
	PGPASS=secret-fake-password # Password!
	HOMEBREW_REPOSITORY=/opt/homebrew # Plain
	QUEUES_REDIS_HOST=127.0.0.2 # Redis!
	QUEUES_REDIS_PORT=6379 # Redis!
	PUBSUB_REDIS_HOST=127.0.0.3 # Redis!
	PUBSUB_REDIS_PORT=6379 # Redis!
	RATELIMITER_REDIS_HOST=127.0.0.4 # Redis
	GCLOUD_2_REDIS_HOST=127.0.0.5 # Redis
	REDIS_PASSWORD=fake-redis-password # Redis!
	REDIS_HOST=localhost # Redis!
	REDIS_PORT=6379 # Redis!
`)

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

		snapshot, err := store.Snapshot()
		require.NoError(t, err)
		require.NotNil(t, snapshot)
	})

	t.Run("Getter with namespaces", func(t *testing.T) {
		store, err := NewStore(withSpecsFile(".env.example", fakeNS, true), WithEnvFile(".env", fakeNS))
		require.NoError(t, err)
		require.NotNil(t, store)

		val, ok, err := store.InsecureGet("REDIS_HOST")
		require.NoError(t, err)
		require.True(t, ok)
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
}

func TestStore_ComplexWithTagValidation(t *testing.T) {
	t.Run("Invalid env values", func(t *testing.T) {
		fake := []byte(`GOPATH=/Users/sourishkrout/go
	INSTRUMENTATION_KEY=05a2cc58-5101-4c69-a0d0-7a126253a972 # Secret!
	HOMEBREW_REPOSITORY=/opt/homebrew # Plain
	REDIS_HOST=12345 # Redis!
	REDIS_PORT=invalid-port # Redis!`)
		store, err := NewStore(withSpecsFile(".env.example", fake, true), WithEnvFile(".env", fake))
		require.NoError(t, err)
		require.NotNil(t, store)

		snapshot, err := store.Snapshot()
		require.NoError(t, err)

		snapshot.sortbyKey()
		assert.EqualValues(t, "REDIS_HOST", snapshot[3].Var.Key)
		assert.EqualValues(t, "12345", snapshot[3].Value.Resolved)
		assert.EqualValues(t,
			`Error 1: The value of variable "REDIS_HOST" failed tag validation "ip|hostname" required by "Redis->HOST" declared in ".env.example"`,
			snapshot[3].Errors[0].Message,
		)

		assert.EqualValues(t, "REDIS_PORT", snapshot[4].Var.Key)
		assert.EqualValues(t, "invalid-port", snapshot[4].Value.Resolved)
		assert.EqualValues(t,
			`Error 1: The value of variable "REDIS_PORT" failed tag validation "number" required by "Redis->PORT" declared in ".env.example"`,
			snapshot[4].Errors[0].Message,
		)
	})
}

func TestStore_ComplexWithDbUrlValidation(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		fake := []byte(`GOPATH=/Users/sourishkrout/go
	DATABASE_URL=postgres://platform:platform@localhost:5432/platform # DatabaseUrl`)
		store, err := NewStore(withSpecsFile(".env.example", fake, true), WithEnvFile(".env", fake))
		require.NoError(t, err)
		require.NotNil(t, store)

		snapshot, err := store.Snapshot()
		require.NoError(t, err)
		snapshot.sortbyKey()
		require.Len(t, snapshot[0].Errors, 0)
	})

	t.Run("Invalid scheme", func(t *testing.T) {
		fake := []byte(`GOPATH=/Users/sourishkrout/go
	DATABASE_URL=abcdef://platform:platform@localhost:5432/platform # DatabaseUrl`)
		store, err := NewStore(withSpecsFile(".env.example", fake, true), WithEnvFile(".env", fake))
		require.NoError(t, err)
		require.NotNil(t, store)

		snapshot, err := store.Snapshot()
		require.NoError(t, err)

		snapshot.sortbyKey()
		assert.EqualValues(t, "DATABASE_URL", snapshot[0].Var.Key)
		assert.EqualValues(t, "abc...orm", snapshot[0].Value.Resolved)
		assert.EqualValues(t,
			`Error 2: The value of variable "DATABASE_URL" failed DatabaseUrl validation "unknown database scheme" required by "DatabaseUrl->URL" declared in ".env.example"`,
			snapshot[0].Errors[0].Message,
		)
	})

	t.Run("Invalid format", func(t *testing.T) {
		fake := []byte(`GOPATH=/Users/sourishkrout/go
	DATABASE_URL=this-is-not-a-database-url # DatabaseUrl`)
		store, err := NewStore(withSpecsFile(".env.example", fake, true), WithEnvFile(".env", fake))
		require.NoError(t, err)
		require.NotNil(t, store)

		snapshot, err := store.Snapshot()
		require.NoError(t, err)

		snapshot.sortbyKey()
		assert.EqualValues(t, "DATABASE_URL", snapshot[0].Var.Key)
		assert.EqualValues(t, "thi...url", snapshot[0].Value.Resolved)
		assert.EqualValues(t,
			`Error 2: The value of variable "DATABASE_URL" failed DatabaseUrl validation "invalid database scheme" required by "DatabaseUrl->URL" declared in ".env.example"`,
			snapshot[0].Errors[0].Message,
		)
	})
}

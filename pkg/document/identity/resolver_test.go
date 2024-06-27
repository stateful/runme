package identity

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stateful/runme/v3/internal/ulid"
)

func TestLifecycleIdentities(t *testing.T) {
	t.Run("Contains", func(t *testing.T) {
		tests := []struct {
			name       string
			identities *LifecycleIdentities
			id         LifecycleIdentity
			expected   bool
		}{
			{"DocumentIdentityTrue", documentIdentities, DocumentLifecycleIdentity, true},
			{"CellIdentityFalse", cellIdentities, DocumentLifecycleIdentity, false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := tt.identities.Contains(tt.id)
				assert.Equal(t, tt.expected, result)
			})
		}
	})
}

func TestIdentityResolver(t *testing.T) {
	t.Run("NewResolver", func(t *testing.T) {
		resolver := NewResolver(DocumentLifecycleIdentity)
		assert.True(t, resolver.DocumentEnabled())
		assert.False(t, resolver.CellEnabled())
	})

	t.Run("CellEnabled", func(t *testing.T) {
		resolver := NewResolver(CellLifecycleIdentity)
		assert.True(t, resolver.CellEnabled())
	})

	t.Run("DocumentEnabled", func(t *testing.T) {
		resolver := NewResolver(DocumentLifecycleIdentity)
		assert.True(t, resolver.DocumentEnabled())
	})

	t.Run("GetCellID_IdentityRequired", func(t *testing.T) {
		id := "01HF53Z4RCVPRANKFBZYMS72QW"
		ulid.MockGenerator(id)
		resolver := NewResolver(CellLifecycleIdentity)
		obj := struct{}{}
		attributes := map[string]string{"id": id}
		id, ok := resolver.GetCellID(obj, attributes)

		assert.True(t, ok)
		assert.NotEmpty(t, id)
	})

	t.Run("GetCellID_IdentityNotRequired", func(t *testing.T) {
		id := "01J1D6BDDD767E819NV8W7YQC2"
		ulid.MockGenerator(id)
		resolver := NewResolver(UnspecifiedLifecycleIdentity)
		obj := struct{}{}
		attributes := map[string]string{"id": id}
		id, ok := resolver.GetCellID(obj, attributes)

		assert.True(t, ok)
		assert.NotEmpty(t, id)
	})
}

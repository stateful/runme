package identity

import (
	"testing"

	parserv1 "github.com/stateful/runme/internal/gen/proto/go/runme/parser/v1"
	identity "github.com/stateful/runme/internal/ulid"
	"github.com/stretchr/testify/assert"
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

	t.Run("GetCellID", func(t *testing.T) {
		ulid := "01HF53Z4RCVPRANKFBZYMS72QW"
		identity.MockGenerator(ulid)
		resolver := NewResolver(CellLifecycleIdentity)
		obj := struct{}{}
		attributes := map[string]string{"id": ulid}
		id, ok := resolver.GetCellID(obj, attributes)

		assert.True(t, ok)
		assert.NotEmpty(t, id)
	})
}

func TestToLifecycleIdentity(t *testing.T) {
	tests := []struct {
		name     string
		idt      parserv1.RunmeIdentity
		expected LifecycleIdentity
	}{
		{"AllIdentity", parserv1.RunmeIdentity_RUNME_IDENTITY_ALL, AllLifecycleIdentity},
		{"DocumentIdentity", parserv1.RunmeIdentity_RUNME_IDENTITY_DOCUMENT, DocumentLifecycleIdentity},
		{"CellIdentity", parserv1.RunmeIdentity_RUNME_IDENTITY_CELL, CellLifecycleIdentity},
		{"UnspecifiedIdentity", parserv1.RunmeIdentity(999), UnspecifiedLifecycleIdentity},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToLifecycleIdentity(tt.idt)
			assert.Equal(t, tt.expected, result)
		})
	}
}

package utils

import (
	"strings"
	"sync"
	"testing"

	"github.com/go-playground/assert/v2"
)

// TestValidID tests the ValidID function with both valid and invalid inputs.
func TestValidID(t *testing.T) {
	// Generate a valid ULID for testing.
	validULID := GenerateID()

	tests := []struct {
		id       string
		expected bool
	}{
		{validULID, true},
		{"0", false},
		{"invalidulid", false},
		{"invalidulid", false},
		{"01B4E6BXY0PRJ5G420D25MWQY!", false},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			if got := ValidID(tt.id); got != tt.expected {
				t.Errorf("ValidID(%s) = %v; want %v", tt.id, got, tt.expected)
			}
		})
	}
}

func TestGenerateID(t *testing.T) {
	id := GenerateID()
	if !ValidID(id) {
		t.Errorf("Generated ID is not a valid ULID: %s", id)
	}

	if len(id) != 26 {
		t.Errorf("Generated ID does not have the correct length: got %v want %v", len(id), 26)
	}

	if strings.ContainsAny(id, "ilou") {
		t.Errorf("Generated ID contains invalid characters: %s", id)
	}
}

func TestGenerateUniqueID(t *testing.T) {
	// inline function to set the generator
	Generator := func() string { return GenerateID() }

	t.Run("uniqueness", func(t *testing.T) {
		id1 := Generator()
		id2 := Generator()
		assert.NotEqual(t, id1, id2)
	})

	t.Run("concurrent uniqueness", func(t *testing.T) {
		var wg sync.WaitGroup
		ids := make(map[string]struct{})
		mu := sync.Mutex{}

		generateAndStoreID := func() {
			defer wg.Done()
			id := Generator()
			mu.Lock()
			defer mu.Unlock()
			ids[id] = struct{}{}
		}

		numIDs := 10000

		wg.Add(numIDs)
		for i := 0; i < numIDs; i++ {
			go generateAndStoreID()
		}

		wg.Wait()

		assert.Equal(t, numIDs, len(ids))
	})
}

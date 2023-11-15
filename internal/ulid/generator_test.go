package ulid

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidID(t *testing.T) {
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
				assert.Equal(t, tt.expected, got)
			}
		})
	}
}

func TestGenerateUniqueID(t *testing.T) {
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

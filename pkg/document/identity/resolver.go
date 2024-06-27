package identity

import (
	"sync"

	"github.com/stateful/runme/v3/internal/ulid"
)

type LifecycleIdentity int

// LifecycleIdentities are used to determine which object identities should be generated.
// The default is to generate all identities.
//
// The following identities are supported:
// - UnspecifiedLifecycleIdentity: No identity is generated.
// - AllLifecycleIdentity: All identities are generated.
// - DocumentLifecycleIdentity: Document identities are generated.
// - CellLifecycleIdentity: Cell identities are generated.
const (
	UnspecifiedLifecycleIdentity LifecycleIdentity = iota
	AllLifecycleIdentity
	DocumentLifecycleIdentity
	CellLifecycleIdentity
)

const DefaultLifecycleIdentity = AllLifecycleIdentity

var documentIdentities = &LifecycleIdentities{
	AllLifecycleIdentity,
	DocumentLifecycleIdentity,
}

var cellIdentities = &LifecycleIdentities{
	AllLifecycleIdentity,
	CellLifecycleIdentity,
}

type LifecycleIdentities []LifecycleIdentity

// Contains returns true if the required identity is contained in the provided identities.
func (ids LifecycleIdentities) Contains(id LifecycleIdentity) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

type IdentityResolver struct {
	documentIdentity bool
	cellIdentity     bool
	cache            *sync.Map
}

func NewResolver(required LifecycleIdentity) *IdentityResolver {
	return &IdentityResolver{
		documentIdentity: documentIdentities.Contains(required),
		cellIdentity:     cellIdentities.Contains(required),
		cache:            &sync.Map{},
	}
}

// CellEnabled returns true if the resolver is configured to generate cell identities.
func (ir *IdentityResolver) CellEnabled() bool {
	return ir.cellIdentity
}

// DocumentEnabled returns true if the resolver is configured to generate document identities.
func (ir *IdentityResolver) DocumentEnabled() bool {
	return ir.documentIdentity
}

// GetCellID returns a cell ID and a boolean indicating if it's new or from attributes.
func (ir *IdentityResolver) GetCellID(obj any, attributes map[string]string) (string, bool) {
	// we used to early return here when !ir.cellIdentity, but we
	// need cell IDs for the ephemeral case too, ie 'runme.dev/id'

	// todo(sebastian): are invalid ulid's valid IDs?
	// Check for a valid 'id' in attributes;
	// if present and valid due to explicit cell identity cache and return it.
	if n, ok := attributes["id"]; ok && ulid.ValidID(n) {
		ir.cache.Store(obj, n)
		return n, true
	}

	if v, ok := ir.cache.Load(obj); ok {
		return v.(string), false
	}

	id := ulid.GenerateID()
	ir.cache.Store(obj, id)

	return id, false
}

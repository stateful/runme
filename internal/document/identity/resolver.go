package identity

import (
	parserv1 "github.com/stateful/runme/internal/gen/proto/go/runme/parser/v1"
	ulid "github.com/stateful/runme/internal/ulid"
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

// LifecycleIdentities is a slice of LifecycleIdentity.
type LifecycleIdentities []LifecycleIdentity

const (
	DefaultLifecycleIdentity = AllLifecycleIdentity
)

var documentIdentities = &LifecycleIdentities{
	AllLifecycleIdentity,
	DocumentLifecycleIdentity,
}

var cellIdentities = &LifecycleIdentities{
	AllLifecycleIdentity,
	CellLifecycleIdentity,
}

// Contains returns true if the required identity is contained in the provided identities.
func (required *LifecycleIdentities) Contains(id LifecycleIdentity) bool {
	for _, v := range *required {
		if v == id {
			return true
		}
	}
	return false
}

// IdentityResolver resolves object identities.
type IdentityResolver struct {
	documentIdentity bool
	cellIdentity     bool
	cache            map[interface{}]string
}

// NewResolver creates a new resolver.
func NewResolver(required LifecycleIdentity) *IdentityResolver {
	return &IdentityResolver{
		documentIdentity: documentIdentities.Contains(required),
		cellIdentity:     cellIdentities.Contains(required),
		cache:            map[interface{}]string{},
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
func (ir *IdentityResolver) GetCellID(obj interface{}, attributes map[string]string) (string, bool) {
	if !ir.cellIdentity {
		return "", false
	}

	// todo(sebastian): are invalid ulid's valid IDs?
	// Check for a valid 'id' in attributes;
	// if present and valid due to explicit cell identity cache and return it.
	if n, ok := attributes["id"]; ok && ulid.ValidID(n) {
		ir.cache[obj] = n
		return n, true
	}

	if v, ok := ir.cache[obj]; ok {
		return v, false
	}

	id := ulid.GenerateID()
	ir.cache[obj] = id

	return id, false
}

// ToLifecycleIdentity converts a parserv1.RunmeIdentity to a LifecycleIdentity.
func ToLifecycleIdentity(idt parserv1.RunmeIdentity) LifecycleIdentity {
	switch idt {
	case parserv1.RunmeIdentity_RUNME_IDENTITY_ALL:
		return AllLifecycleIdentity
	case parserv1.RunmeIdentity_RUNME_IDENTITY_DOCUMENT:
		return DocumentLifecycleIdentity
	case parserv1.RunmeIdentity_RUNME_IDENTITY_CELL:
		return CellLifecycleIdentity
	default:
		return UnspecifiedLifecycleIdentity
	}
}

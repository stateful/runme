package identity

import (
	parserv1 "github.com/stateful/runme/internal/gen/proto/go/runme/parser/v1"
	ulid "github.com/stateful/runme/internal/ulid"
)

type LifecycleIdentity int

const (
	UnspecifiedLifecycleIdentity LifecycleIdentity = iota
	AllLifecycleIdentity
	DocumentLifecycleIdentity
	CellLifecycleIdentity
)

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

func (required *LifecycleIdentities) Contains(id LifecycleIdentity) bool {
	for _, v := range *required {
		if v == id {
			return true
		}
	}
	return false
}

type IdentityResolver struct {
	documentIdentity bool
	cellIdentity     bool
	cache            map[interface{}]string
}

func NewResolver(required LifecycleIdentity) *IdentityResolver {
	return &IdentityResolver{
		documentIdentity: documentIdentities.Contains(required),
		cellIdentity:     cellIdentities.Contains(required),
		cache:            map[interface{}]string{},
	}
}

func (ir *IdentityResolver) CellEnabled() bool {
	return ir.cellIdentity
}

func (ir *IdentityResolver) DocumentEnabled() bool {
	return ir.documentIdentity
}

func (ir *IdentityResolver) GetCellId(obj interface{}, attributes map[string]string) (string, bool) {
	if !ir.cellIdentity {
		return "", false
	}

	if n, ok := attributes["id"]; ok && n != "" {
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

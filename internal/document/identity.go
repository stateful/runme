package document

type identityResolver interface {
	GetCellID(obj any, attributes map[string]string) (string, bool)
}

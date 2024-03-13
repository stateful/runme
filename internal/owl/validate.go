package owl

import "fmt"

type ValidationError interface {
	fmt.Stringer
	VarItem() *SetVarItem
	Error() string
	Message() string
	Key() string
	SpecName() string
	Source() string
	Code() ValidateErrorType
}

type ValidationErrors []ValidationError

type ValidateErrorType int

const (
	ValidateErrorVarRequired ValidateErrorType = iota
)

type RequiredError struct {
	varItem *SetVarItem
	code    ValidateErrorType
}

func NewRequiredError(varItem *SetVarItem) *RequiredError {
	return &RequiredError{
		varItem: varItem,
		code:    ValidateErrorVarRequired,
	}
}

func (e RequiredError) VarItem() *SetVarItem {
	return e.varItem
}

func (e RequiredError) Error() string {
	return fmt.Sprintf("Error %v: Variable \"%s\" is unresolved but defined as required by \"%s!\" in \"%s\"",
		e.Code(),
		e.Key(),
		e.SpecName(),
		e.Source())
}

func (e RequiredError) Message() string {
	return e.Error()
}

func (e RequiredError) String() string {
	return e.Error()
}

func (e RequiredError) Code() ValidateErrorType {
	return e.code
}

func (e RequiredError) Key() string {
	return e.varItem.Var.Key
}

func (e RequiredError) SpecName() string {
	return e.varItem.Spec.Name
}

func (e RequiredError) Source() string {
	if e.varItem.Var.Operation == nil {
		return "-"
	}
	return e.varItem.Var.Operation.Source
}

// make sure interfaces are satisfied
var (
	_ ValidationError = new(RequiredError)
	_ error           = new(RequiredError)
)

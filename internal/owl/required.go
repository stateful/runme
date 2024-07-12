package owl

import "fmt"

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
	return fmt.Sprintf("Error %v: Variable \"%s\" is unresolved but declared as required by \"%s!\" in \"%s\"",
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
	if e.varItem.Spec.Operation == nil {
		return "-"
	}
	if e.varItem.Spec.Operation.Source == "" {
		return "-"
	}
	return e.varItem.Spec.Operation.Source
}

// make sure interfaces are satisfied
var (
	_ ValidationError = new(RequiredError)
	_ error           = new(RequiredError)
)

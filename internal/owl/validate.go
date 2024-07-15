package owl

import (
	"fmt"
	"strings"

	valid "github.com/go-playground/validator/v10"
)

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

type ValidateErrorType uint8

const (
	ValidateErrorVarRequired ValidateErrorType = iota
	ValidateErrorTagFailed
)

type TagFailedError struct {
	varItem *SetVarItem
	code    ValidateErrorType
	tag     string
	item    string
}

func NewTagFailedError(varItem *SetVarItem, tag string, item string) *TagFailedError {
	return &TagFailedError{
		varItem: varItem,
		code:    ValidateErrorTagFailed,
		tag:     tag,
		item:    item,
	}
}

func (e TagFailedError) VarItem() *SetVarItem {
	return e.varItem
}

func (e TagFailedError) Error() string {
	return fmt.Sprintf("Error %v: The value of variable \"%s\" failed tag validation \"%s\" required by \"%s->%s\" declared in \"%s\"",
		e.Code(),
		e.Key(),
		e.Tag(),
		e.SpecName(),
		e.Item(),
		e.Source())
}

func (e TagFailedError) Message() string {
	return e.Error()
}

func (e TagFailedError) String() string {
	return e.Error()
}

func (e TagFailedError) Code() ValidateErrorType {
	return e.code
}

func (e TagFailedError) Key() string {
	return e.varItem.Var.Key
}

func (e TagFailedError) Tag() string {
	return e.tag
}

func (e TagFailedError) SpecName() string {
	return e.varItem.Spec.Name
}

func (e TagFailedError) Item() string {
	return e.item
}

func (e TagFailedError) Source() string {
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
	_ ValidationError = new(TagFailedError)
	_ error           = new(TagFailedError)
)

const ComplexSpecType string = "Complex"

type ComplexDef struct {
	Name    string
	Breaker string
	Items   map[string]*varSpec
}

var validator = valid.New()

var ComplexDefTypes = map[string]*ComplexDef{
	"Redis": {
		Name:    "Redis",
		Breaker: "REDIS",
		Items: map[string]*varSpec{
			"HOST": {
				Name:     SpecNamePlain,
				Rules:    "ip|hostname",
				Required: true,
			},
			"PORT": {
				Name:     SpecNamePlain,
				Rules:    "number",
				Required: true,
			},
			"PASSWORD": {
				Name:     SpecNamePassword,
				Rules:    "min=18,max=32",
				Required: false,
			},
		},
	},
	"Postgres": {
		Name:    "Postgres",
		Breaker: "POSTGRES",
		Items: map[string]*varSpec{
			"HOST": {
				Name:     SpecNamePlain,
				Rules:    "required,ip|hostname",
				Required: true,
			},
		},
	},
}

func (s *ComplexOperationSet) validate() (ValidationErrors, error) {
	var validationErrs ValidationErrors

	for _, k := range s.Keys {
		spec, ok := s.specs[k]
		if !ok {
			return nil, fmt.Errorf("spec not found for key: %s", k)
		}

		if spec.Var.Key != k {
			continue
		}

		typ, ok := ComplexDefTypes[spec.Spec.Name]
		if !ok {
			return nil, fmt.Errorf("complex type not found: %s", spec.Spec.Name)
		}

		val, ok := s.values[spec.Var.Key]
		if !ok {
			return nil, fmt.Errorf("value not found for key: %s", spec.Var.Key)
		}

		parts := strings.Split(val.Var.Key, typ.Breaker+"_")
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid key not matching complex item: %s", val.Var.Key)
		}

		itemKey := (parts[len(parts)-1])
		item, ok := typ.Items[itemKey]
		if !ok {
			return nil, fmt.Errorf("complex item not found: %s", itemKey)
		}

		data := make(map[string]interface{}, 1)
		rules := make(map[string]interface{}, 1)

		if val.Value.Resolved == "" && !item.Required {
			continue
		}

		data[val.Var.Key] = val.Value.Resolved
		rules[val.Var.Key] = item.Rules

		field := validator.ValidateMap(data, rules)

		for key, errs := range field {
			verrs, ok := errs.(valid.ValidationErrors)
			if !ok {
				return nil, fmt.Errorf("unexpected error type: %T", errs)
			}
			for _, err := range verrs {
				val := s.values[key]
				spec := s.specs[key]
				validationErrs = append(validationErrs,
					NewTagFailedError(
						&SetVarItem{
							Var:   val.Var,
							Value: val.Value,
							Spec:  spec.Spec,
						},
						err.Tag(),
						itemKey,
					),
				)
			}
		}
	}

	return validationErrs, nil
}

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
}

func NewTagFailedError(varItem *SetVarItem, tag string) *TagFailedError {
	return &TagFailedError{
		varItem: varItem,
		code:    ValidateErrorTagFailed,
		tag:     tag,
	}
}

func (e TagFailedError) VarItem() *SetVarItem {
	return e.varItem
}

func (e TagFailedError) Error() string {
	return fmt.Sprintf("Error %v: The value of variable \"%s\" failed tag validation \"%s\" required by \"%s!\" declared in \"%s\"",
		e.Code(),
		e.Key(),
		e.tag,
		e.SpecName(),
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

func (e TagFailedError) SpecName() string {
	return e.varItem.Spec.Name
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

type SpecDef struct {
	Name    string
	Breaker string
	Items   map[string]*varSpec
}

var validator = valid.New()

var SpecDefTypes = map[string]*SpecDef{
	"Redis": {
		Name:    "Redis",
		Breaker: "REDIS",
		Items: map[string]*varSpec{
			"HOST": {
				Name:     SpecNamePlain,
				Rules:    "required,ip|hostname",
				Required: true,
			},
			"PORT": {
				Name:     SpecNamePlain,
				Rules:    "required,number",
				Required: true,
			},
			"PASSWORD": {
				Name:     SpecNamePassword,
				Rules:    "required,min=18,max=32",
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
				Rules:    "required,min=4,max=32",
				Required: true,
			},
		},
	},
}

func (s *ComplexOperationSet) validate() (ValidationErrors, error) {
	data := make(map[string]interface{})
	rules := make(map[string]interface{})
	for _, k := range s.Keys {
		spec, ok := s.specs[k]
		if !ok {
			// should these be errors?
			continue
		}

		if spec.Var.Key != k {
			// should these be errors?
			continue
		}

		typ, ok := SpecDefTypes[spec.Spec.Name]
		if !ok {
			// should these be errors?
			continue
		}

		val, ok := s.values[spec.Var.Key]
		if !ok {
			// should these be errors?
			continue
		}

		parts := strings.Split(val.Var.Key, typ.Breaker+"_")
		if len(parts) < 2 {
			// should these be errors?
			continue
		}

		typkey := (parts[len(parts)-1])
		data[val.Var.Key] = val.Value.Resolved
		rules[val.Var.Key] = typ.Items[typkey].Rules
	}

	fields := validator.ValidateMap(data, rules)
	var validationErrs ValidationErrors

	for key, errs := range fields {
		verrs, ok := errs.(valid.ValidationErrors)
		if !ok {
			return nil, fmt.Errorf("unexpected error type: %T", errs)
		}
		for _, err := range verrs {
			val := s.values[key]
			spec := s.specs[key]
			validationErrs = append(validationErrs,
				NewTagFailedError(
					&SetVarItem{Var: val.Var, Value: val.Value, Spec: spec.Spec},
					err.Tag(),
				),
			)
		}
	}

	return validationErrs, nil
}

package owl

import (
	"fmt"
	"strings"

	"github.com/xo/dburl"

	valid "github.com/go-playground/validator/v10"
)

// todo(sebastian): perhaps this should be ValueError instead?
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

//revive:disable:var-naming
const (
	ValidateErrorVarRequired ValidateErrorType = iota
	ValidateErrorTagFailed
	ValidateErrorResolutionFailed
	// ValidateErrorDatabaseUrl
	// ValidateErrorJwtFailed
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
	return e.varItem.Spec.Spec
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

type ResolutionFailedError struct {
	varItem *SetVarItem
	err     error
	code    ValidateErrorType
	item    string
}

func NewResolutionFailedError(varItem *SetVarItem, item string, err error) *ResolutionFailedError {
	return &ResolutionFailedError{
		code:    ValidateErrorResolutionFailed,
		err:     err,
		item:    item,
		varItem: varItem,
	}
}

func (e ResolutionFailedError) VarItem() *SetVarItem {
	return e.varItem
}

func (e ResolutionFailedError) Error() string {
	return fmt.Sprintf("Error %v: The value of variable \"%s\" failed resolution \"%s\" required by \"%s->%s\" declared in \"%s\"",
		e.Code(),
		e.Key(),
		e.err.Error(),
		e.SpecName(),
		e.Item(),
		e.Source())
}

func (e ResolutionFailedError) Message() string {
	return e.Error()
}

func (e ResolutionFailedError) String() string {
	return e.Error()
}

func (e ResolutionFailedError) Code() ValidateErrorType {
	return e.code
}

func (e ResolutionFailedError) Key() string {
	return e.varItem.Var.Key
}

func (e ResolutionFailedError) SpecName() string {
	return e.varItem.Spec.Spec
}

func (e ResolutionFailedError) Item() string {
	return e.item
}

func (e ResolutionFailedError) Source() string {
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
	_ ValidationError = new(ResolutionFailedError)
	_ error           = new(ResolutionFailedError)
)

// make sure interfaces are satisfied
var (
	_ ValidationError = new(TagFailedError)
	_ error           = new(TagFailedError)
)

const SpecTypeKey string = "Spec"

var validator *valid.Validate

func init() {
	validator = valid.New()
	if err := validator.RegisterValidation("database_url", func(fl valid.FieldLevel) bool {
		if _, err := dburl.Parse(fl.Field().String()); err != nil {
			return false
		}
		return true
	}); err != nil {
		panic(err)
	}
}

type SpecDef struct {
	Name      string              `json:"name"`
	Breaker   string              `json:"breaker"`
	Atomics   map[string]*varSpec `json:"atomics" yaml:"-"`
	Validator func(item *varSpec, itemKey string, varItem *SetVarItem) (ValidationErrors, error)
}

func (cd *SpecDef) Validate(itemKey string, varItem *SetVarItem) (ValidationErrors, error) {
	itemAtomic, ok := cd.Atomics[itemKey]
	if !ok {
		return nil, fmt.Errorf("spec item not found: %s", itemKey)
	}

	if varItem.Value.Resolved == "" && !itemAtomic.Required {
		return nil, nil
	}

	return cd.Validator(itemAtomic, itemKey, varItem)
}

func TagValidator(item *varSpec, itemKey string, varItem *SetVarItem) (ValidationErrors, error) {
	data := make(map[string]interface{}, 1)
	rules := make(map[string]interface{}, 1)

	data[varItem.Var.Key] = varItem.Value.Resolved
	rules[varItem.Var.Key] = item.Rules

	field := validator.ValidateMap(data, rules)

	var validationErrs ValidationErrors
	for _, errs := range field {
		verrs, ok := errs.(valid.ValidationErrors)
		if !ok {
			return nil, fmt.Errorf("unexpected error type: %T", errs)
		}
		for _, err := range verrs {
			validationErrs = append(validationErrs,
				NewTagFailedError(
					&SetVarItem{
						Var:   varItem.Var,
						Value: varItem.Value,
						Spec:  varItem.Spec,
					},
					err.Tag(),
					itemKey,
				),
			)
		}
	}

	return validationErrs, nil
}

func (s *SpecOperationSet) validate(specDefs SpecDefs) (ValidationErrors, error) {
	var validationErrs ValidationErrors

	for _, k := range s.Keys {
		spec, ok := s.specs[k]
		if !ok {
			return nil, fmt.Errorf("spec not found for key: %s", k)
		}

		if spec.Var.Key != k {
			continue
		}

		specType, ok := specDefs[spec.Spec.Name]
		if !ok {
			return nil, fmt.Errorf("spec type not found: %s", spec.Spec.Name)
		}

		akey, aitem, err := s.GetAtomic(spec, specDefs)
		if err != nil {
			return nil, err
		}

		verrs, err := specType.Validate(
			akey,
			aitem)
		if err != nil {
			return nil, err
		}
		validationErrs = append(validationErrs, verrs...)
	}

	return validationErrs, nil
}

func (s *SpecOperationSet) GetAtomic(spec *SetVarSpec, specDefs SpecDefs) (string, *SetVarItem, error) {
	val, ok := s.values[spec.Var.Key]
	if !ok {
		return "", nil, fmt.Errorf("value not found for key: %s", spec.Var.Key)
	}

	specType, ok := specDefs[spec.Spec.Name]
	if !ok {
		return spec.Var.Key, &SetVarItem{
			Var:   spec.Var,
			Value: val.Value,
			Spec:  spec.Spec,
		}, nil
	}

	varKeyParts := strings.Split(val.Var.Key, specType.Breaker+"_")
	if len(varKeyParts) < 2 {
		return "", nil, fmt.Errorf("invalid key not matching spec item: %s", val.Var.Key)
	}

	varKey := (varKeyParts[len(varKeyParts)-1])
	varNS := (varKeyParts[0])

	item, ok := specType.Atomics[varKey]
	if !ok {
		return "", nil, fmt.Errorf("spec missing atomic for %s", varKey)
	}

	aspec := *item
	// aspec := *spec.Spec
	aspec.Spec = spec.Spec.Name
	aspec.Namespace = varNS

	return varKey, &SetVarItem{
		Var:   val.Var,
		Value: val.Value,
		Spec:  &aspec,
	}, nil
}

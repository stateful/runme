package owl

import (
	"fmt"
	"strings"

	valid "github.com/go-playground/validator/v10"
	"github.com/xo/dburl"
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

//revive:disable:var-naming
const (
	ValidateErrorVarRequired ValidateErrorType = iota
	ValidateErrorTagFailed
	ValidateErrorDatabaseUrl
	// ValidateErrorJwtFailed
)

type DatabaseUrlError struct {
	varItem *SetVarItem
	code    ValidateErrorType
	item    string
	error   error
}

func NewDatabaseUrlError(varItem *SetVarItem, err error, item string) *DatabaseUrlError {
	return &DatabaseUrlError{
		varItem: varItem,
		code:    ValidateErrorDatabaseUrl,
		item:    item,
		error:   err,
	}
}

//revive:enable:var-naming

func (e DatabaseUrlError) VarItem() *SetVarItem {
	return e.varItem
}

func (e DatabaseUrlError) Error() string {
	return fmt.Sprintf("Error %v: The value of variable \"%s\" failed DatabaseUrl validation \"%s\" required by \"%s->%s\" declared in \"%s\"",
		e.Code(),
		e.Key(),
		e.error.Error(),
		e.SpecName(),
		e.Item(),
		e.Source())
}

func (e DatabaseUrlError) Message() string {
	return e.Error()
}

func (e DatabaseUrlError) String() string {
	return e.Error()
}

func (e DatabaseUrlError) Code() ValidateErrorType {
	return e.code
}

func (e DatabaseUrlError) Key() string {
	return e.varItem.Var.Key
}

func (e DatabaseUrlError) SpecName() string {
	return e.varItem.Spec.Complex
}

func (e DatabaseUrlError) Item() string {
	return e.item
}

func (e DatabaseUrlError) Source() string {
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
	_ ValidationError = new(DatabaseUrlError)
	_ error           = new(DatabaseUrlError)
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
	return e.varItem.Spec.Complex
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

// type JwtFailedError struct {
// 	varItem *SetVarItem
// 	code    ValidateErrorType
// 	item    string
// 	reason  string
// }

// func NewJwtFailedError(varItem *SetVarItem, item string, reason string) *JwtFailedError {
// 	return &JwtFailedError{
// 		varItem: varItem,
// 		code:    ValidateErrorJwtFailed,
// 		item:    item,
// 		reason:  reason,
// 	}
// }

// func (e JwtFailedError) VarItem() *SetVarItem {
// 	return e.varItem
// }

// func (e JwtFailedError) Error() string {
// 	return fmt.Sprintf("Error %v: The value of variable \"%s\" failed JWT validation (%s) required by \"%s->%s\" declared in \"%s\"",
// 		e.Code(),
// 		e.Key(),
// 		e.Reason(),
// 		e.SpecName(),
// 		e.Item(),
// 		e.Source())
// }

// func (e JwtFailedError) Message() string {
// 	return e.Error()
// }

// func (e JwtFailedError) String() string {
// 	return e.Error()
// }

// func (e JwtFailedError) Code() ValidateErrorType {
// 	return e.code
// }

// func (e JwtFailedError) Key() string {
// 	return e.varItem.Var.Key
// }

// func (e JwtFailedError) Reason() string {
// 	return e.reason
// }

// func (e JwtFailedError) SpecName() string {
// 	return e.varItem.Spec.Complex
// }

// func (e JwtFailedError) Item() string {
// 	return e.item
// }

// func (e JwtFailedError) Source() string {
// 	if e.varItem.Spec.Operation == nil {
// 		return "-"
// 	}
// 	if e.varItem.Spec.Operation.Source == "" {
// 		return "-"
// 	}
// 	return e.varItem.Spec.Operation.Source
// }

// // make sure interfaces are satisfied
// var (
// 	_ ValidationError = new(JwtFailedError)
// 	_ error           = new(JwtFailedError)
// )

const ComplexSpecType string = "Complex"

var validator = valid.New()

type ComplexDef struct {
	Name      string
	Breaker   string
	Items     map[string]*varSpec
	Validator func(item *varSpec, itemKey string, varItem *SetVarItem) (ValidationErrors, error)
}

func (cd *ComplexDef) Validate(itemKey string, varItem *SetVarItem) (ValidationErrors, error) {
	complexItem, ok := cd.Items[itemKey]
	if !ok {
		return nil, fmt.Errorf("complex item not found: %s", itemKey)
	}

	if varItem.Value.Resolved == "" && !complexItem.Required {
		return nil, nil
	}

	return cd.Validator(complexItem, itemKey, varItem)
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

func DatabaseValidator(item *varSpec, itemKey string, varItem *SetVarItem) (ValidationErrors, error) {
	var validationErrs ValidationErrors

	_, err := dburl.Parse(varItem.Value.Resolved)
	if err != nil {
		validationErrs = append(validationErrs,
			NewDatabaseUrlError(
				&SetVarItem{
					Var:   varItem.Var,
					Value: varItem.Value,
					Spec:  varItem.Spec,
				},
				err,
				itemKey,
			))
	}

	return validationErrs, nil
}

var ComplexDefTypes = map[string]*ComplexDef{
	"Auth0": {
		Name:    "Auth0",
		Breaker: "AUTH0",
		Items: map[string]*varSpec{
			"AUDIENCE": {
				Name:     AtomicNamePlain,
				Rules:    "url",
				Required: true,
			},
			"CLIENT_ID": {
				Name:     AtomicNamePlain,
				Rules:    "alphanum,min=32,max=32",
				Required: true,
			},
			"DOMAIN": {
				Name:     AtomicNamePlain,
				Rules:    "fqdn",
				Required: true,
			},
		},
		Validator: TagValidator,
	},
	"Auth0Mgmt": {
		Name:    "Auth0Mgmt",
		Breaker: "AUTH0_MANAGEMENT",
		Items: map[string]*varSpec{
			"CLIENT_ID": {
				Name:     AtomicNamePlain,
				Rules:    "alphanum,min=32,max=32",
				Required: true,
			},
			"CLIENT_SECRET": {
				Name:     AtomicNameSecret,
				Rules:    "ascii,min=64,max=64",
				Required: true,
			},
			"AUDIENCE": {
				Name:     AtomicNamePlain,
				Rules:    "url",
				Required: true,
			},
		},
		Validator: TagValidator,
	},
	"DatabaseUrl": {
		Name:    "DatabaseUrl",
		Breaker: "DATABASE",
		Items: map[string]*varSpec{
			"URL": {
				Name:     AtomicNameSecret,
				Rules:    "url",
				Required: true,
			},
		},
		Validator: DatabaseValidator,
	},
	"OpenAI": {
		Name:    "OpenAI",
		Breaker: "OPENAI",
		Items: map[string]*varSpec{
			"ORG_ID": {
				Name:     AtomicNamePlain,
				Rules:    "ascii,min=28,max=28,startswith=org-",
				Required: true,
			},
			"API_KEY": {
				Name:     AtomicNameSecret,
				Rules:    "ascii,min=34,startswith=sk-",
				Required: true,
			},
		},
		Validator: TagValidator,
	},
	"Redis": {
		Name:    "Redis",
		Breaker: "REDIS",
		Items: map[string]*varSpec{
			"HOST": {
				Name:     AtomicNamePlain,
				Rules:    "ip|hostname",
				Required: true,
			},
			"PORT": {
				Name:     AtomicNamePlain,
				Rules:    "number",
				Required: true,
			},
			"PASSWORD": {
				Name:     AtomicNamePassword,
				Rules:    "min=18,max=32",
				Required: false,
			},
		},
		Validator: TagValidator,
	},
	"Slack": {
		Name:    "Slack",
		Breaker: "SLACK",
		Items: map[string]*varSpec{
			"CLIENT_ID": {
				Name:     AtomicNamePlain,
				Rules:    "min=24,max=24",
				Required: true,
			},
			"CLIENT_SECRET": {
				Name:     AtomicNameSecret,
				Rules:    "min=32,max=32",
				Required: true,
			},
			"REDIRECT_URL": {
				Name:     AtomicNameSecret,
				Rules:    "url",
				Required: true,
			},
		},
		Validator: TagValidator,
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

		complexType, ok := ComplexDefTypes[spec.Spec.Name]
		if !ok {
			return nil, fmt.Errorf("complex type not found: %s", spec.Spec.Name)
		}

		akey, aitem, err := s.GetAtomicItem(spec)
		if err != nil {
			return nil, err
		}

		verrs, err := complexType.Validate(
			akey,
			aitem)
		if err != nil {
			return nil, err
		}
		validationErrs = append(validationErrs, verrs...)
	}

	return validationErrs, nil
}

func (s *ComplexOperationSet) GetAtomicItem(spec *SetVarSpec) (string, *SetVarItem, error) {
	val, ok := s.values[spec.Var.Key]
	if !ok {
		return "", nil, fmt.Errorf("value not found for key: %s", spec.Var.Key)
	}

	complexType, ok := ComplexDefTypes[spec.Spec.Name]
	if !ok {
		return spec.Var.Key, &SetVarItem{
			Var:   spec.Var,
			Value: val.Value,
			Spec:  spec.Spec,
		}, nil
	}

	varKeyParts := strings.Split(val.Var.Key, complexType.Breaker+"_")
	if len(varKeyParts) < 2 {
		return "", nil, fmt.Errorf("invalid key not matching complex item: %s", val.Var.Key)
	}

	varKey := (varKeyParts[len(varKeyParts)-1])
	varNS := (varKeyParts[0])

	item := complexType.Items[varKey]

	aspec := *spec.Spec
	aspec.Complex = aspec.Name
	aspec.Name = item.Name
	aspec.Namespace = varNS

	return varKey, &SetVarItem{
		Var:   val.Var,
		Value: val.Value,
		Spec:  &aspec,
	}, nil
}

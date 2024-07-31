package owl

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/graphql-go/graphql"
)

// Constants representing different spec names.
// These constants are of type SpecName and are assigned string values.
const (
	SpecNameOpaque   string = "Opaque"   // SpecNameOpaque specifies an opaque specification.
	SpecNamePlain    string = "Plain"    // SpecNamePlain specifies a plain specification.
	SpecNameSecret   string = "Secret"   // SpecNameSecret specifies a secret specification.
	SpecNamePassword string = "Password" // SpecNamePassword specifies a password specification.
	SpecNameDefault         = SpecNameOpaque
)

type specType struct {
	typeName   string
	typeObject *graphql.Object
	resolveFn  graphql.FieldResolveFn
}

var (
	Schema      graphql.Schema
	SpecTypes   map[string]*specType
	ComplexType *specType
)

var EnvironmentType,
	ValidateType,
	RenderType,
	SpecTypeErrorsType *graphql.Object

// todo(sebastian): use gql interface?
func registerSpecFields(fields graphql.Fields) {
	for _, t := range SpecTypes {
		fields[t.typeName] = &graphql.Field{
			Type:    t.typeObject,
			Resolve: t.resolveFn,
			Args: graphql.FieldConfigArgument{
				"keys": &graphql.ArgumentConfig{
					Type: graphql.NewList(graphql.String),
				},
				"insecure": &graphql.ArgumentConfig{
					Type:         graphql.Boolean,
					DefaultValue: false,
				},
			},
		}
	}

	fields[ComplexType.typeName] = &graphql.Field{
		Type:    ComplexType.typeObject,
		Resolve: ComplexType.resolveFn,
		Args: graphql.FieldConfigArgument{
			"name": &graphql.ArgumentConfig{
				Type: graphql.NewNonNull(graphql.String),
			},
			"namespace": &graphql.ArgumentConfig{
				Type: graphql.NewNonNull(graphql.String),
			},
			"keys": &graphql.ArgumentConfig{
				Type: graphql.NewList(graphql.String),
			},
		},
	}
}

func registerSpec(name string, sensitive, mask bool, resolver graphql.FieldResolveFn) *specType {
	typ := graphql.NewObject(graphql.ObjectConfig{
		Name: fmt.Sprintf("SpecType%s", name),
		Fields: (graphql.FieldsThunk)(func() graphql.Fields {
			fields := graphql.Fields{
				"name": &graphql.Field{
					Type: graphql.String,
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						return name, nil
					},
				},
				"sensitive": &graphql.Field{
					Type: graphql.Boolean,
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						return sensitive, nil
					},
				},
				"mask": &graphql.Field{
					Type: graphql.Boolean,
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						return mask, nil
					},
				},
				"errors": &graphql.Field{
					Type: graphql.NewList(SpecTypeErrorsType),
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						var opSet *OperationSet

						switch p.Source.(type) {
						case *OperationSet:
							opSet = p.Source.(*OperationSet)
						case *ComplexOperationSet:
							opSet = p.Source.(*ComplexOperationSet).OperationSet
						default:
							return nil, errors.New("source does not contain an OperationSet")
						}

						// todo(sebastian): move into interface?
						var verrs []*SetVarError
						for _, spec := range opSet.specs {
							if spec.Spec.Error == nil {
								continue
							}

							code := spec.Spec.Error.Code()
							verr := &SetVarError{
								Code:    int(code),
								Message: spec.Spec.Error.Message(),
							}
							verrs = append(verrs, verr)
						}

						return verrs, nil
					},
				},
				"done": &graphql.Field{
					Type: EnvironmentType,
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						return p.Source, nil
					},
				},
			}

			registerSpecFields(fields)

			return fields
		}),
	})

	return &specType{
		typeName:   name,
		typeObject: typ,
		resolveFn:  resolver,
	}
}

func registerComplexType(resolver graphql.FieldResolveFn) *specType {
	name := ComplexSpecType
	typ := graphql.NewObject(graphql.ObjectConfig{
		Name: fmt.Sprintf("SpecType%s", name),
		Fields: (graphql.FieldsThunk)(func() graphql.Fields {
			fields := graphql.Fields{
				"name": &graphql.Field{
					Type: graphql.String,
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						return name, nil
					},
				},
				"errors": &graphql.Field{
					Type: graphql.NewList(SpecTypeErrorsType),
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						var opSet *OperationSet

						switch p.Source.(type) {
						case *OperationSet:
							opSet = p.Source.(*OperationSet)
						case *ComplexOperationSet:
							opSet = p.Source.(*ComplexOperationSet).OperationSet
						default:
							return nil, errors.New("source does not contain an OperationSet")
						}

						// todo(sebastian): move into interface?
						var verrs []*SetVarError
						for _, spec := range opSet.specs {
							if spec.Spec.Error == nil {
								continue
							}

							code := spec.Spec.Error.Code()
							verr := &SetVarError{
								Code:    int(code),
								Message: spec.Spec.Error.Message(),
							}
							verrs = append(verrs, verr)
						}

						return verrs, nil
					},
				},
				"done": &graphql.Field{
					Type: EnvironmentType,
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						return p.Source, nil
					},
				},
			}

			registerSpecFields(fields)

			return fields
		}),
	})

	return &specType{
		typeName:   name,
		typeObject: typ,
		resolveFn:  resolver,
	}
}

type SpecResolverMutator func(val *SetVarValue, spec *SetVarSpec, insecure bool)

func specResolver(mutator SpecResolverMutator) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		insecure := p.Args["insecure"].(bool)
		keysArg := p.Args["keys"].([]interface{})

		var opSet *OperationSet
		complexName := ""
		complexNs := ""

		switch p.Source.(type) {
		case *OperationSet:
			opSet = p.Source.(*OperationSet)
			complexName = ""
			complexNs = ""
		case *ComplexOperationSet:
			opSet = p.Source.(*ComplexOperationSet).OperationSet
			complexName = p.Source.(*ComplexOperationSet).Name
			complexNs = p.Source.(*ComplexOperationSet).Namespace
		default:
			return nil, errors.New("source does not contain an OperationSet")
		}

		for _, kArg := range keysArg {
			k := kArg.(string)
			val, valOk := opSet.values[k]
			spec, specOk := opSet.specs[k]
			if !valOk && !specOk {
				// todo(sebastian): superfluous keys are only possible in hand-written queries
				continue
			}

			spec.Spec.Complex = complexName
			spec.Spec.Namespace = complexNs

			// skip if last known status was DELETED
			if valOk && val.Value.Status == "DELETED" {
				continue
			}

			// todo(sebastian): poc, move to something more generic
			if valOk && val.Value.Status == "UNRESOLVED" {
				if !specOk {
					// todo(sebastian): without spec, we can't decide if unresolved is valid - should be impossible
					continue
				}

				if spec.Spec.Required {
					spec.Spec.Error = NewRequiredError(&SetVarItem{Var: val.Var, Value: val.Value, Spec: spec.Spec})
					continue
				}

				if val.Value.Operation != nil && val.Value.Operation.Kind == ReconcileSetOperation {
					continue
				}
			}

			mutator(val, spec, insecure)
			if specOk {
				spec.Spec.Checked = true
			}
		}

		return p.Source, nil
	}
}

func resolveSensitiveKeys() graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		sensitive := SetVarItems{}
		var opSet *OperationSet

		switch p.Source.(type) {
		case nil, string:
			// root passes string
			return sensitive, nil
		case *OperationSet:
			opSet = p.Source.(*OperationSet)
		case *ComplexOperationSet:
			opSet = p.Source.(*ComplexOperationSet).OperationSet
		default:
			return nil, errors.New("source does not contain an OperationSet")
		}

		for _, v := range opSet.values {
			s, ok := opSet.specs[v.Var.Key]
			if !ok {
				return nil, fmt.Errorf("missing spec for %s", v.Var.Key)
			}

			item := &SetVarItem{
				Var:  v.Var,
				Spec: s.Spec,
			}

			sensitive = append(sensitive, item)
		}

		sensitive.sort()
		return sensitive, nil
	}
}

func resolveDotEnv() graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		insecure := p.Args["insecure"].(bool)
		prefix := p.Args["prefix"].(string)
		dotenv := SetVarItems{}
		var opSet *OperationSet

		switch p.Source.(type) {
		case nil, string:
			// root passes string
			return dotenv, nil
		case *OperationSet:
			opSet = p.Source.(*OperationSet)
		case *ComplexOperationSet:
			opSet = p.Source.(*ComplexOperationSet).OperationSet
		default:
			return nil, errors.New("source does not contain an OperationSet")
		}

		var buf bytes.Buffer
		// todo(sebastian): this should really be up the graph
		for _, v := range opSet.values {
			switch insecure {
			case true:
				if v.Value.Status == "UNRESOLVED" {
					continue
				}
				if v.Value.Status == "DELETED" {
					continue
				}
			case false:
				if v.Value.Status != "LITERAL" {
					continue
				}
			}

			_, _ = buf.WriteString(fmt.Sprintf("%s%s=\"%s\"\n", prefix, v.Var.Key, v.Value.Resolved))
		}

		return buf.String(), nil
	}
}

func resolveGetter() graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		key := p.Args["key"].(string)
		kv := &SetVarItem{}
		var opSet *OperationSet

		switch p.Source.(type) {
		case nil, string:
			// root passes string
			return kv, nil
		case *OperationSet:
			opSet = p.Source.(*OperationSet)
		case *ComplexOperationSet:
			opSet = p.Source.(*ComplexOperationSet).OperationSet
		default:
			return nil, errors.New("source is does not contain an OperationSet")
		}

		val, ok := opSet.values[key]
		if !ok {
			return kv, nil
		}

		kv.Var = val.Var
		kv.Value = val.Value

		spec, ok := opSet.specs[key]
		if ok {
			kv.Spec = spec.Spec
		}

		return kv, nil
	}
}

func resolveSnapshot() graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		insecure := p.Args["insecure"].(bool)

		snapshot := SetVarItems{}
		var opSet *OperationSet

		switch p.Source.(type) {
		case nil, string:
			// root passes string
			return snapshot, nil
		case *OperationSet:
			opSet = p.Source.(*OperationSet)
		case *ComplexOperationSet:
			opSet = p.Source.(*ComplexOperationSet).OperationSet
		default:
			return nil, errors.New("source does not contain an OperationSet")
		}

		// todo(sebastian): this should really be up the graph
		for _, v := range opSet.values {
			switch insecure {
			case true:
				if v.Value.Status == "UNRESOLVED" {
					continue
				}
				if v.Value.Status == "DELETED" {
					continue
				}
			case false:
				if v.Value.Status == "DELETED" {
					v.Value.Original = ""
					v.Value.Resolved = ""
					v.Value.Status = "UNRESOLVED"
				}
			}
			s, ok := opSet.specs[v.Var.Key]
			if !ok {
				return nil, fmt.Errorf("missing spec for %s", v.Var.Key)
			}

			item := &SetVarItem{
				Var:   v.Var,
				Value: v.Value,
				Spec:  s.Spec,
			}
			if s.Spec != nil && s.Spec.Error != nil {
				code := s.Spec.Error.Code()
				item.Errors = append(item.Errors, &SetVarError{
					Code:    int(code),
					Message: s.Spec.Error.Message(),
				})
			}

			snapshot = append(snapshot, item)
		}

		snapshot.sort()
		return snapshot, nil
	}
}

func resolveOperation(resolveMutator func(SetVarItems, *OperationSet, bool) error) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		vars, ok := p.Args["vars"]
		if !ok {
			return p.Source, nil
		}
		// location := p.Args["location"].(string)
		hasSpecs := p.Args["hasSpecs"].(bool)

		var resolverOpSet *OperationSet
		var err error

		switch p.Source.(type) {
		case *OperationSet:
			resolverOpSet = p.Source.(*OperationSet)
			resolverOpSet.hasSpecs = resolverOpSet.hasSpecs || hasSpecs
		default:
			resolverOpSet, err = NewOperationSet(WithOperation(TransientSetOperation))
			resolverOpSet.hasSpecs = hasSpecs
			if err != nil {
				return nil, err
			}
		}

		buf, err := json.Marshal(vars)
		if err != nil {
			return nil, err
		}

		var revive SetVarItems
		err = json.Unmarshal(buf, &revive)
		if err != nil {
			return nil, err
		}

		err = resolveMutator(revive, resolverOpSet, hasSpecs)
		if err != nil {
			return nil, err
		}

		return resolverOpSet, nil
	}
}

func mutateLoadOrUpdate(revived SetVarItems, resolverOpSet *OperationSet, hasSpecs bool) error {
	for _, r := range revived {
		source := ""
		if r.Value != nil {
			if r.Value.Operation != nil {
				source = r.Value.Operation.Source
			}
			newCreated := r.Var.Created
			if old, ok := resolverOpSet.values[r.Var.Key]; ok {
				oldCreated := old.Var.Created
				r.Var.Created = oldCreated
				if old.Var.Origin != "" {
					source = old.Var.Origin
				} else if old.Value.Operation != nil && old.Value.Operation.Kind != ReconcileSetOperation {
					source = old.Value.Operation.Source
				}
			}
			r.Var.Origin = source
			r.Var.Updated = newCreated
			if r.Value.Original != "" {
				r.Value.Resolved = r.Value.Original
				r.Value.Status = "LITERAL"
			} else {
				// todo(sebastian): load vs update difference?
				r.Value.Status = "UNRESOLVED"
			}
			resolverOpSet.values[r.Var.Key] = &SetVarValue{Var: r.Var, Value: r.Value}
		}

		if r.Spec != nil {
			if r.Spec.Operation != nil {
				source = r.Spec.Operation.Source
			}
			newCreated := r.Var.Created
			if old, ok := resolverOpSet.specs[r.Var.Key]; ok {
				oldCreated := *old.Var.Created
				r.Var.Created = &oldCreated
				if old.Spec.Operation != nil {
					source = old.Spec.Operation.Source
				}
			}
			r.Var.Origin = source
			r.Var.Updated = newCreated
			resolverOpSet.specs[r.Var.Key] = &SetVarSpec{Var: r.Var, Spec: r.Spec}
		}
	}
	return nil
}

func mutateDelete(vars SetVarItems, resolverOpSet *OperationSet, _ bool) error {
	for _, v := range vars {
		val, vOk := resolverOpSet.values[v.Var.Key]
		if !vOk {
			val = &SetVarValue{v.Var, v.Value}
		}
		val.Value.Status = "DELETED"
		val.Value.Resolved = ""
		resolverOpSet.values[v.Var.Key] = val

		// spec, sOk := resolverOpSet.specs[v.Var.Key]
		// if sOk || v.Spec == nil {
		// 	continue
		// }
		// spec = &SetVarSpec{Var: v.Var, Spec: v.Spec}
		// resolverOpSet.specs[v.Var.Key] = spec
	}
	return nil
}

func init() {
	SpecTypes = make(map[string]*specType)

	SpecTypes[SpecNameSecret] = registerSpec(SpecNameSecret, true, true,
		specResolver(func(val *SetVarValue, spec *SetVarSpec, insecure bool) {
			if insecure {
				original := val.Value.Original
				val.Value.Resolved = original
				val.Value.Status = "LITERAL"
				return
			}

			val.Value.Status = "MASKED"
			original := val.Value.Original
			val.Value.Original = ""
			val.Value.Resolved = ""
			if len(original) > 24 {
				val.Value.Resolved = original[:3] + "..." + original[len(original)-3:]
			}
		}),
	)

	SpecTypes[SpecNamePassword] = registerSpec(SpecNamePassword, true, true,
		specResolver(func(val *SetVarValue, spec *SetVarSpec, insecure bool) {
			if insecure {
				original := val.Value.Original
				val.Value.Resolved = original
				val.Value.Status = "LITERAL"
				return
			}

			val.Value.Status = "MASKED"
			original := val.Value.Original
			val.Value.Original = ""
			val.Value.Resolved = strings.Repeat("*", max(8, len(original)))
		}),
	)
	SpecTypes[SpecNameOpaque] = registerSpec(SpecNameOpaque, true, false,
		specResolver(func(val *SetVarValue, spec *SetVarSpec, insecure bool) {
			if insecure {
				original := val.Value.Original
				val.Value.Resolved = original
				val.Value.Status = "LITERAL"
				return
			}

			val.Value.Status = "HIDDEN"
			val.Value.Resolved = ""
		}),
	)
	SpecTypes[SpecNamePlain] = registerSpec(SpecNamePlain, false, false,
		specResolver(func(val *SetVarValue, spec *SetVarSpec, insecure bool) {
			if insecure {
				original := val.Value.Original
				val.Value.Resolved = original
				val.Value.Status = "LITERAL"
				return
			}

			val.Value.Resolved = val.Value.Original
			val.Value.Status = "LITERAL"
		}),
	)

	ComplexType = registerComplexType(
		func(p graphql.ResolveParams) (interface{}, error) {
			name := p.Args["name"].(string)
			ns := p.Args["namespace"].(string)
			keys := p.Args["keys"].([]interface{})

			var complexOpSet *ComplexOperationSet

			switch p.Source.(type) {
			case *OperationSet:
				complexOpSet = &ComplexOperationSet{
					OperationSet: p.Source.(*OperationSet),
					Name:         name,
					Namespace:    ns,
				}
			case *ComplexOperationSet:
				complexOpSet = p.Source.(*ComplexOperationSet)
			default:
				return nil, errors.New("source does not contain an OperationSet")
			}

			var valuekeys []string
			for _, k := range keys {
				v, ok := k.(string)
				if !ok {
					continue
				}
				valuekeys = append(valuekeys, v)
			}

			complexOpSet.Name = name
			complexOpSet.Namespace = ns
			complexOpSet.Keys = valuekeys

			validationErrs, err := complexOpSet.validate()
			if err != nil {
				return nil, err
			}

			for _, verr := range validationErrs {
				key := verr.VarItem().Var.Key
				complexOpSet.specs[key].Spec.Error = verr
			}

			return complexOpSet, nil
		})

	SpecTypeErrorsType = graphql.NewObject(graphql.ObjectConfig{
		Name: "SpecTypeErrorsType",
		Fields: graphql.Fields{
			"message": &graphql.Field{
				Type: graphql.String,
			},
			"code": &graphql.Field{
				Type: graphql.Int,
			},
		},
	})

	ValidateType = graphql.NewObject(graphql.ObjectConfig{
		Name: "ValidateType",
		Fields: (graphql.FieldsThunk)(func() graphql.Fields {
			fields := graphql.Fields{
				"done": &graphql.Field{
					Type: EnvironmentType,
				},
			}
			registerSpecFields(fields)
			return fields
		}),
	})

	OperationType := &graphql.Field{
		Type: graphql.NewObject(graphql.ObjectConfig{
			Name: "VariableOperationType",
			Fields: graphql.Fields{
				"order": &graphql.Field{
					Type: graphql.Int,
				},

				"kind": &graphql.Field{
					Type: graphql.Int,
				},

				"source": &graphql.Field{
					Type: graphql.String,
				},
			},
		}),
	}
	VariableType := graphql.NewObject(
		graphql.ObjectConfig{
			Name: "VariableType",
			Fields: graphql.Fields{
				"var": &graphql.Field{
					Type: graphql.NewObject(graphql.ObjectConfig{
						Name: "VariableVarType",
						Fields: graphql.Fields{
							"key": &graphql.Field{
								Type: graphql.String,
							},
							"origin": &graphql.Field{
								Type: graphql.String,
							},
							"created": &graphql.Field{
								Type: graphql.DateTime,
							},
							"updated": &graphql.Field{
								Type: graphql.DateTime,
							},
							"operation": OperationType,
						},
					}),
				},
				"value": &graphql.Field{
					Type: graphql.NewObject(graphql.ObjectConfig{
						Name: "VariableValueType",
						Fields: graphql.Fields{
							// "type": &graphql.Field{
							// 	Type: graphql.String,
							// },
							"original": &graphql.Field{
								Type: graphql.String,
							},
							"resolved": &graphql.Field{
								Type: graphql.String,
							},
							"status": &graphql.Field{
								Type: graphql.String,
							},
							"operation": OperationType,
							// "success": &graphql.Field{
							// 	Type: graphql.Boolean,
							// },
							// "validationErrors": &graphql.Field{
							// 	Type: graphql.NewList(graphql.String),
							// },
						},
					}),
				},
				"spec": &graphql.Field{
					Type: graphql.NewObject(graphql.ObjectConfig{
						Name: "VariableSpecType",
						Fields: graphql.Fields{
							"name": &graphql.Field{
								Type: graphql.String,
							},
							"description": &graphql.Field{
								Type: graphql.String,
							},
							"required": &graphql.Field{
								Type: graphql.Boolean,
							},
							"checked": &graphql.Field{
								Type: graphql.Boolean,
							},
							"operation": OperationType,
						},
					}),
				},
				"errors": &graphql.Field{
					Type: graphql.NewList(SpecTypeErrorsType),
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						vars, ok := p.Source.(*SetVarItem)
						if !ok {
							return nil, errors.New("source is not a *SetVarItem")
						}

						return vars.Errors, nil
					},
				},
			},
		},
	)

	RenderType = graphql.NewObject(graphql.ObjectConfig{
		Name: "RenderType",
		Fields: (graphql.FieldsThunk)(func() graphql.Fields {
			return graphql.Fields{
				"snapshot": &graphql.Field{
					Type: graphql.NewNonNull(graphql.NewList(VariableType)),
					Args: graphql.FieldConfigArgument{
						"insecure": &graphql.ArgumentConfig{
							Type:         graphql.Boolean,
							DefaultValue: false,
						},
					},
					Resolve: resolveSnapshot(),
				},
				"dotenv": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
					Args: graphql.FieldConfigArgument{
						"insecure": &graphql.ArgumentConfig{
							Type:         graphql.Boolean,
							DefaultValue: false,
						},
						"prefix": &graphql.ArgumentConfig{
							Type:         graphql.String,
							DefaultValue: "",
						},
					},
					Resolve: resolveDotEnv(),
				},
				"get": &graphql.Field{
					Type: graphql.NewNonNull(VariableType),
					Args: graphql.FieldConfigArgument{
						"key": &graphql.ArgumentConfig{
							Type:         graphql.String,
							DefaultValue: "",
						},
					},
					Resolve: resolveGetter(),
				},
				"sensitiveKeys": &graphql.Field{
					Type:    graphql.NewNonNull(graphql.NewList(VariableType)),
					Resolve: resolveSensitiveKeys(),
				},
			}
		}),
	})

	OperationInputType := &graphql.InputObjectFieldConfig{
		Type: graphql.NewInputObject(graphql.InputObjectConfig{
			Name: "VariableOperationInput",
			Fields: graphql.InputObjectConfigFieldMap{
				"order": &graphql.InputObjectFieldConfig{
					Type: graphql.Int,
				},
				"kind": &graphql.InputObjectFieldConfig{
					Type: graphql.Int,
				},
				"source": &graphql.InputObjectFieldConfig{
					Type: graphql.String,
				},
			},
		}),
	}

	VariableInputType := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "VariableInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"var": &graphql.InputObjectFieldConfig{
				Type: graphql.NewInputObject(graphql.InputObjectConfig{
					Name: "VariableVarInput",
					Fields: graphql.InputObjectConfigFieldMap{
						"key": &graphql.InputObjectFieldConfig{
							Type: graphql.String,
						},
						"created": &graphql.InputObjectFieldConfig{
							Type: graphql.DateTime,
						},
						"updated": &graphql.InputObjectFieldConfig{
							Type: graphql.DateTime,
						},
						"operation": OperationInputType,
					},
				}),
			},
			"value": &graphql.InputObjectFieldConfig{
				Type: graphql.NewInputObject(graphql.InputObjectConfig{
					Name: "VariableValueInput",
					Fields: graphql.InputObjectConfigFieldMap{
						// "type": &graphql.InputObjectFieldConfig{
						// 	Type: graphql.String,
						// },
						"original": &graphql.InputObjectFieldConfig{
							Type: graphql.String,
						},
						"resolved": &graphql.InputObjectFieldConfig{
							Type: graphql.String,
						},
						"status": &graphql.InputObjectFieldConfig{
							Type: graphql.String,
						},
						"operation": OperationInputType,
						// "success": &graphql.InputObjectFieldConfig{
						// 	Type: graphql.Boolean,
						// },
					},
				}),
			},
			"spec": &graphql.InputObjectFieldConfig{
				Type: graphql.NewInputObject(graphql.InputObjectConfig{
					Name: "VariableSpecInput",
					Fields: graphql.InputObjectConfigFieldMap{
						"name": &graphql.InputObjectFieldConfig{
							Type: graphql.String,
						},
						"description": &graphql.InputObjectFieldConfig{
							Type: graphql.String,
						},
						"required": &graphql.InputObjectFieldConfig{
							Type:         graphql.Boolean,
							DefaultValue: false,
						},
						"operation": OperationInputType,
						"checked": &graphql.InputObjectFieldConfig{
							Type:         graphql.Boolean,
							DefaultValue: false,
						},
					},
				}),
			},
		},
	})

	EnvironmentType = graphql.NewObject(graphql.ObjectConfig{
		Name: "EnvironmentType",
		Fields: (graphql.FieldsThunk)(func() graphql.Fields {
			return graphql.Fields{
				"load": &graphql.Field{
					Type: EnvironmentType,
					Args: graphql.FieldConfigArgument{
						"vars": &graphql.ArgumentConfig{
							Type: graphql.NewList(VariableInputType),
						},
						"hasSpecs": &graphql.ArgumentConfig{
							Type:         graphql.Boolean,
							DefaultValue: false,
						},
						// "location": &graphql.ArgumentConfig{
						// 	Type:         graphql.String,
						// 	DefaultValue: "",
						// },
					},
					Resolve: resolveOperation(mutateLoadOrUpdate),
				},
				"reconcile": &graphql.Field{
					Type: EnvironmentType,
					Args: graphql.FieldConfigArgument{
						"vars": &graphql.ArgumentConfig{
							Type: graphql.NewList(VariableInputType),
						},
						"hasSpecs": &graphql.ArgumentConfig{
							Type:         graphql.Boolean,
							DefaultValue: false,
						},
						// "location": &graphql.ArgumentConfig{
						// 	Type:         graphql.String,
						// 	DefaultValue: "",
						// },
					},
					Resolve: resolveOperation(mutateLoadOrUpdate),
				},
				"update": &graphql.Field{
					Type: EnvironmentType,
					Args: graphql.FieldConfigArgument{
						"vars": &graphql.ArgumentConfig{
							Type: graphql.NewList(VariableInputType),
						},
						"hasSpecs": &graphql.ArgumentConfig{
							Type:         graphql.Boolean,
							DefaultValue: false,
						},
						// "location": &graphql.ArgumentConfig{
						// 	Type:         graphql.String,
						// 	DefaultValue: "",
						// },
					},
					Resolve: resolveOperation(mutateLoadOrUpdate),
				},
				"delete": &graphql.Field{
					Type: EnvironmentType,
					Args: graphql.FieldConfigArgument{
						"vars": &graphql.ArgumentConfig{
							Type: graphql.NewList(VariableInputType),
						},
						"hasSpecs": &graphql.ArgumentConfig{
							Type:         graphql.Boolean,
							DefaultValue: false,
						},
						// "location": &graphql.ArgumentConfig{
						// 	Type:         graphql.String,
						// 	DefaultValue: "",
						// },
					},
					Resolve: resolveOperation(mutateDelete),
				},
				"validate": &graphql.Field{
					Type: ValidateType,
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						return p.Source, nil
					},
				},
				"render": &graphql.Field{
					Type: RenderType,
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						return p.Source, nil
					},
				},
			}
		}),
	})

	SpecsListType := graphql.NewObject(graphql.ObjectConfig{
		Name: "SpecsListType",
		Fields: (graphql.FieldsThunk)(func() graphql.Fields {
			return graphql.Fields{
				"list": &graphql.Field{
					Type: graphql.NewList(graphql.NewObject(graphql.ObjectConfig{
						Name: "SpecListType",
						Fields: graphql.Fields{
							"name": &graphql.Field{
								Type: graphql.String,
							},
							// todo(sebastian): should be enum
							"sensitive": &graphql.Field{
								Type: graphql.Boolean,
							},
							"mask": &graphql.Field{
								Type: graphql.Boolean,
							},
						},
					})),
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						var keys []map[string]string
						for k := range SpecTypes {
							keys = append(keys, map[string]string{"name": k})
						}

						return keys, nil
					},
				},
			}
		}),
	})

	var err error
	Schema, err = graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(
			graphql.ObjectConfig{
				Name: "Query",
				Fields: graphql.Fields{
					"environment": &graphql.Field{
						Type: EnvironmentType,
						Resolve: func(p graphql.ResolveParams) (interface{}, error) {
							return p.Info.FieldName, nil
						},
					},
					"specs": &graphql.Field{
						Type: SpecsListType,
						Resolve: func(p graphql.ResolveParams) (interface{}, error) {
							return p.Info.FieldName, nil
						},
					},
				},
			},
		),
	})
	if err != nil {
		// inconsistent schema is bad
		panic(err)
	}
}

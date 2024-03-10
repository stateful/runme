package owl

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
)

type specType struct {
	typ     *graphql.Object
	resolve graphql.FieldResolveFn
}

var (
	Schema                        graphql.Schema
	EnvironmentType, ValidateType *graphql.Object
	SpecTypes                     map[string]*specType
)

type QueryNodeReducer func(*ast.OperationDefinition, *ast.SelectionSet) (*ast.SelectionSet, error)

// todo(sebastian): use gql interface?
func registerSpecFields(fields graphql.Fields) {
	for k, v := range SpecTypes {
		fields[k] = &graphql.Field{
			Type:    v.typ,
			Resolve: v.resolve,
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
}

func registerSpec(spec string, sensitive, mask bool, resolver graphql.FieldResolveFn) *specType {
	typ := graphql.NewObject(graphql.ObjectConfig{
		Name: fmt.Sprintf("SpecType%s", spec),
		Fields: (graphql.FieldsThunk)(func() graphql.Fields {
			fields := graphql.Fields{
				"spec": &graphql.Field{
					Type: graphql.String,
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						return spec, nil
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
					Type: graphql.NewList(graphql.String),
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
		typ:     typ,
		resolve: resolver,
	}
}

func specResolver(mutator func(*SetVarValue, *SetVarSpec, bool)) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		insecure := p.Args["insecure"].(bool)
		keysArg := p.Args["keys"].([]interface{})

		opSet := p.Source.(*OperationSet)
		for _, kArg := range keysArg {
			k := kArg.(string)
			v, vOk := opSet.values[k]
			s, sOk := opSet.specs[k]
			if !vOk && !sOk {
				// todo(sebastian): should UNSET/delete ever come in here?
				continue
			}

			if vOk && v.Value.Status == "UNRESOLVED" {
				// todo(sebastian): most obvious validation error?
				continue
			}

			// TODO: Specs
			mutator(v, s, insecure)
		}

		return p.Source, nil
	}
}

func init() {
	SpecTypes = make(map[string]*specType)

	SpecTypes[SpecNameSecret] = registerSpec(SpecNameSecret, true, true,
		specResolver(func(v *SetVarValue, s *SetVarSpec, insecure bool) {
			if insecure {
				original := v.Value.Original
				v.Value.Resolved = original
				v.Value.Status = "LITERAL"
				return
			}

			v.Value.Status = "MASKED"
			original := v.Value.Original
			v.Value.Original = ""
			if len(original) > 24 {
				v.Value.Resolved = original[:3] + "..." + original[len(original)-3:]
			}
		}),
	)

	SpecTypes[SpecNamePassword] = registerSpec(SpecNamePassword, true, true,
		specResolver(func(v *SetVarValue, s *SetVarSpec, insecure bool) {
			if insecure {
				original := v.Value.Original
				v.Value.Resolved = original
				v.Value.Status = "LITERAL"
				return
			}

			v.Value.Status = "MASKED"
			original := v.Value.Original
			v.Value.Original = ""
			v.Value.Resolved = strings.Repeat("*", max(8, len(original)))
		}),
	)
	SpecTypes[SpecNameOpaque] = registerSpec(SpecNameOpaque, true, false,
		specResolver(func(v *SetVarValue, s *SetVarSpec, insecure bool) {
			if insecure {
				original := v.Value.Original
				v.Value.Resolved = original
				v.Value.Status = "LITERAL"
				return
			}

			v.Value.Status = "HIDDEN"
			v.Value.Resolved = ""
		}),
	)
	SpecTypes[SpecNamePlain] = registerSpec(SpecNamePlain, false, false,
		specResolver(func(v *SetVarValue, s *SetVarSpec, insecure bool) {
			if insecure {
				original := v.Value.Original
				v.Value.Resolved = original
				v.Value.Status = "LITERAL"
				return
			}

			v.Value.Resolved = v.Value.Original
			v.Value.Status = "LITERAL"
		}),
	)

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
							"created": &graphql.Field{
								Type: graphql.DateTime,
							},
							"updated": &graphql.Field{
								Type: graphql.DateTime,
							},
							"operation": &graphql.Field{
								Type: graphql.NewObject(graphql.ObjectConfig{
									Name: "VariableOperationType",
									Fields: graphql.Fields{
										"order": &graphql.Field{
											Type: graphql.Int,
										},
										// todo(sebastian): should be enum
										"kind": &graphql.Field{
											Type: graphql.String,
										},
										// todo(sebastian): likely abstract
										"location": &graphql.Field{
											Type: graphql.String,
										},
									},
								}),
							},
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
						},
					}),
				},
			},
		},
	)

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
						"operation": &graphql.InputObjectFieldConfig{
							Type: graphql.NewInputObject(graphql.InputObjectConfig{
								Name: "VariableOperationInput",
								Fields: graphql.InputObjectConfigFieldMap{
									"order": &graphql.InputObjectFieldConfig{
										Type: graphql.Int,
									},
									"kind": &graphql.InputObjectFieldConfig{
										Type: graphql.String,
									},
									"location": &graphql.InputObjectFieldConfig{
										Type: graphql.String,
									},
								},
							}),
						},
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
						"location": &graphql.ArgumentConfig{
							Type:         graphql.String,
							DefaultValue: "",
						},
					},
					Resolve: resolveOperation(resolveLoadOrUpdate),
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
						"location": &graphql.ArgumentConfig{
							Type:         graphql.String,
							DefaultValue: "",
						},
					},
					Resolve: resolveOperation(resolveLoadOrUpdate),
				},
				"update": &graphql.Field{
					Type: EnvironmentType,
					Args: graphql.FieldConfigArgument{
						"vars": &graphql.ArgumentConfig{
							Type: graphql.NewList(VariableInputType),
						},
						"location": &graphql.ArgumentConfig{
							Type:         graphql.String,
							DefaultValue: "",
						},
						"hasSpecs": &graphql.ArgumentConfig{
							Type:         graphql.Boolean,
							DefaultValue: false,
						},
					},
					Resolve: resolveOperation(resolveLoadOrUpdate),
				},
				"delete": &graphql.Field{
					Type: EnvironmentType,
					Args: graphql.FieldConfigArgument{
						"vars": &graphql.ArgumentConfig{
							Type: graphql.NewList(VariableInputType),
						},
						"location": &graphql.ArgumentConfig{
							Type:         graphql.String,
							DefaultValue: "",
						},
						"hasSpecs": &graphql.ArgumentConfig{
							Type:         graphql.Boolean,
							DefaultValue: false,
						},
					},
					Resolve: resolveOperation(resolveDelete),
				},
				"validate": &graphql.Field{
					Type: ValidateType,
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						return p.Source, nil
					},
				},
				"location": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						// todo(sebastian): bring this back?
						switch p.Source.(type) {
						case *OperationSet:
							opSet := p.Source.(*OperationSet)
							return opSet.operation.location, nil
						default:
							// noop
						}

						return nil, nil
					},
				},
				"snapshot": &graphql.Field{
					Type: graphql.NewNonNull(graphql.NewList(VariableType)),
					Args: graphql.FieldConfigArgument{
						"insecure": &graphql.ArgumentConfig{
							Type:         graphql.Boolean,
							DefaultValue: false,
						},
					},
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						insecure := p.Args["insecure"].(bool)

						snapshot := SetVarItems{}
						var opSet *OperationSet

						switch p.Source.(type) {
						case nil, string:
							// root passes string
							return snapshot, nil
						case *OperationSet:
							opSet = p.Source.(*OperationSet)
						default:
							return nil, errors.New("source is not an OperationSet")
						}

						for _, v := range opSet.values {
							if insecure && v.Value.Status == "UNRESOLVED" {
								continue
							}
							s, ok := opSet.specs[v.Var.Key]
							if !ok {
								return nil, fmt.Errorf("missing spec for %s", v.Var.Key)
							}
							snapshot = append(snapshot, &SetVarItem{
								Var:   v.Var,
								Value: v.Value,
								Spec:  s.Spec,
							})
						}

						snapshot.sort()
						return snapshot, nil
					},
				},
			}
		}),
	})

	SpecsType := graphql.NewObject(graphql.ObjectConfig{
		Name: "SpecsType",
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
						Type: SpecsType,
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

func resolveOperation(resolveMutator func(SetVarItems, *OperationSet, string, bool) error) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		vars, ok := p.Args["vars"]
		if !ok {
			return p.Source, nil
		}
		location := p.Args["location"].(string)
		hasSpecs := p.Args["hasSpecs"].(bool)

		var resolverOpSet *OperationSet
		var err error

		switch p.Source.(type) {
		case *OperationSet:
			resolverOpSet = p.Source.(*OperationSet)
			resolverOpSet.hasSpecs = resolverOpSet.hasSpecs || hasSpecs
		default:
			resolverOpSet, err = NewOperationSet(WithOperation(TransientSetOperation, "resolver"))
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

		err = resolveMutator(revive, resolverOpSet, location, hasSpecs)
		if err != nil {
			return nil, err
		}

		return resolverOpSet, nil
	}
}

func reduceSetOperations(store *Store, vars io.StringWriter) QueryNodeReducer {
	return func(opDef *ast.OperationDefinition, selSet *ast.SelectionSet) (*ast.SelectionSet, error) {
		opSetData := make(map[string]SetVarItems, len(store.opSets))

		// TODO: Specs
		for i, opSet := range store.opSets {
			if len(opSet.values) == 0 && len(opSet.specs) == 0 {
				continue
			}

			opName := ""
			switch opSet.operation.kind {
			case LoadSetOperation:
				opName = "load"
			case UpdateSetOperation:
				opName = "update"
			case ReconcileSetOperation:
				opName = "reconcile"
			case DeleteSetOperation:
				opName = "delete"
			default:
				continue
			}
			nvars := fmt.Sprintf("%s_%d", opName, i)

			for _, v := range opSet.values {
				opSetData[nvars] = append(opSetData[nvars], &SetVarItem{
					Var:   v.Var,
					Value: v.Value,
				})
			}

			for _, s := range opSet.specs {
				opSetData[nvars] = append(opSetData[nvars], &SetVarItem{
					Var:  s.Var,
					Spec: s.Spec,
				})
			}

			opDef.VariableDefinitions = append(opDef.VariableDefinitions, ast.NewVariableDefinition(&ast.VariableDefinition{
				Variable: ast.NewVariable(&ast.Variable{
					Name: ast.NewName(&ast.Name{
						Value: nvars,
					}),
				}),
				Type: ast.NewNamed(&ast.Named{
					Name: ast.NewName(&ast.Name{
						Value: "[VariableInput]!",
					}),
				}),
			}))

			nextSelSet := ast.NewSelectionSet(&ast.SelectionSet{})
			// nextSelSet.Selections = append(nextSelSet.Selections, ast.NewField(&ast.Field{
			// 	Name: ast.NewName(&ast.Name{
			// 		Value: "location",
			// 	}),
			// }))
			selSet.Selections = append(selSet.Selections, ast.NewField(&ast.Field{
				Name: ast.NewName(&ast.Name{
					Value: opName,
				}),
				Arguments: []*ast.Argument{
					ast.NewArgument(&ast.Argument{
						Name: ast.NewName(&ast.Name{
							Value: "vars",
						}),
						Value: ast.NewVariable(&ast.Variable{
							Name: ast.NewName(&ast.Name{
								Value: nvars,
							}),
						}),
					}),
					ast.NewArgument(&ast.Argument{
						Name: ast.NewName(&ast.Name{
							Value: "location",
						}),
						Value: ast.NewStringValue(&ast.StringValue{
							Value: opSet.operation.location,
						}),
					}),
					ast.NewArgument(&ast.Argument{
						Name: ast.NewName(&ast.Name{
							Value: "hasSpecs",
						}),
						Value: ast.NewBooleanValue(&ast.BooleanValue{
							Value: opSet.hasSpecs,
						}),
					}),
				},
				Directives:   []*ast.Directive{},
				SelectionSet: nextSelSet,
			}))
			selSet = nextSelSet
		}

		opSetJSON, err := json.MarshalIndent(opSetData, "", " ")
		if err != nil {
			return nil, err
		}
		_, err = vars.WriteString(string(opSetJSON))
		if err != nil {
			return nil, err
		}

		return selSet, nil
	}
}

func reduceSnapshot() QueryNodeReducer {
	return func(opDef *ast.OperationDefinition, selSet *ast.SelectionSet) (*ast.SelectionSet, error) {
		nextSelSet := ast.NewSelectionSet(&ast.SelectionSet{
			Selections: []ast.Selection{
				ast.NewField(&ast.Field{
					Name: ast.NewName(&ast.Name{
						Value: "var",
					}),
					SelectionSet: ast.NewSelectionSet(&ast.SelectionSet{
						Selections: []ast.Selection{
							ast.NewField(&ast.Field{
								Name: ast.NewName(&ast.Name{
									Value: "key",
								}),
							}),
							ast.NewField(&ast.Field{
								Name: ast.NewName(&ast.Name{
									Value: "created",
								}),
							}),
							ast.NewField(&ast.Field{
								Name: ast.NewName(&ast.Name{
									Value: "updated",
								}),
							}),
							ast.NewField(&ast.Field{
								Name: ast.NewName(&ast.Name{
									Value: "operation",
								}),
								SelectionSet: ast.NewSelectionSet(&ast.SelectionSet{
									Selections: []ast.Selection{
										ast.NewField(&ast.Field{
											Name: ast.NewName(&ast.Name{
												Value: "location",
											}),
										}),
									},
								}),
							}),
						},
					}),
				}),
				ast.NewField(&ast.Field{
					Name: ast.NewName(&ast.Name{
						Value: "value",
					}),
					SelectionSet: ast.NewSelectionSet(&ast.SelectionSet{
						Selections: []ast.Selection{
							// ast.NewField(&ast.Field{
							// 	Name: ast.NewName(&ast.Name{
							// 		Value: "type",
							// 	}),
							// }),
							ast.NewField(&ast.Field{
								Name: ast.NewName(&ast.Name{
									Value: "original",
								}),
							}),
							ast.NewField(&ast.Field{
								Name: ast.NewName(&ast.Name{
									Value: "resolved",
								}),
							}),
							ast.NewField(&ast.Field{
								Name: ast.NewName(&ast.Name{
									Value: "status",
								}),
							}),
						},
					}),
				}),
				ast.NewField(&ast.Field{
					Name: ast.NewName(&ast.Name{
						Value: "spec",
					}),
					SelectionSet: ast.NewSelectionSet(&ast.SelectionSet{
						Selections: []ast.Selection{
							ast.NewField(&ast.Field{
								Name: ast.NewName(&ast.Name{
									Value: "name",
								}),
							}),
							ast.NewField(&ast.Field{
								Name: ast.NewName(&ast.Name{
									Value: "required",
								}),
							}),
						},
					}),
				}),
			},
		})

		selSet.Selections = append(selSet.Selections,
			ast.NewField(&ast.Field{
				Name: ast.NewName(&ast.Name{
					Value: "snapshot",
				}),
				Arguments: []*ast.Argument{
					ast.NewArgument(&ast.Argument{
						Name: ast.NewName(&ast.Name{
							Value: "insecure",
						}),
						Value: ast.NewVariable(&ast.Variable{
							Name: ast.NewName(&ast.Name{
								Value: "insecure",
							}),
						}),
					}),
				},
				SelectionSet: nextSelSet,
			}),
		)
		return nextSelSet, nil
	}
}

func reconcileAsymmetry(store *Store) QueryNodeReducer {
	return func(opDef *ast.OperationDefinition, selSet *ast.SelectionSet) (*ast.SelectionSet, error) {
		allSpecs := make(map[string]bool)
		for _, opSet := range store.opSets {
			for k := range opSet.specs {
				allSpecs[k] = true
			}
		}
		allVals := make(map[string]bool)
		for _, opSet := range store.opSets {
			for k := range opSet.values {
				allVals[k] = true
			}
		}

		deltaOpSet, err := NewOperationSet(WithOperation(ReconcileSetOperation, ""))
		if err != nil {
			return nil, err
		}

		for _, opSet := range store.opSets {
			for k := range opSet.values {
				if _, exists := allSpecs[k]; exists {
					continue
				}
				created := time.Now()
				spec := &SetVarSpec{
					Var: &SetVar{
						Key:     k,
						Created: &created,
					},
					Spec: &varSpec{
						Name:     SpecNameDefault,
						Required: false,
						Checked:  false,
					},
				}
				deltaOpSet.specs[k] = spec
			}
			for k := range opSet.specs {
				if _, exists := allVals[k]; exists {
					continue
				}
				created := time.Now()
				spec := &SetVarValue{
					Var: &SetVar{
						Key:     k,
						Created: &created,
					},
					Value: &varValue{
						Status: "UNRESOLVED",
					},
				}
				deltaOpSet.values[k] = spec
			}
		}

		if len(deltaOpSet.specs) > 0 || len(deltaOpSet.values) > 0 {
			deltaOpSet.hasSpecs = len(deltaOpSet.specs) > 0
			store.opSets = append(store.opSets, deltaOpSet)
		}

		return selSet, nil
	}
}

func reduceSepcs(store *Store) QueryNodeReducer {
	return func(opDef *ast.OperationDefinition, selSet *ast.SelectionSet) (*ast.SelectionSet, error) {
		var specKeys []string
		varSpecs := make(map[string]*SetVarItem)
		for _, opSet := range store.opSets {
			if len(opSet.specs) == 0 {
				continue
			}
			for _, s := range opSet.specs {
				if _, ok := SpecTypes[s.Spec.Name]; !ok {
					return nil, fmt.Errorf("unknown spec type: %s on %s", s.Spec.Name, s.Var.Key)
				}
				varSpecs[s.Var.Key] = &SetVarItem{
					Var:  s.Var,
					Spec: s.Spec,
				}
				specKeys = append(specKeys, s.Spec.Name)
			}
		}

		nextVarSpecs := func(varSpecs map[string]*SetVarItem, spec string, prevSelSet *ast.SelectionSet) *ast.SelectionSet {
			var keys []string
			for _, v := range varSpecs {
				if v.Spec.Name != spec {
					continue
				}
				keys = append(keys, v.Var.Key)
			}
			if len(keys) == 0 {
				return prevSelSet
			}

			nextSelSet := ast.NewSelectionSet(&ast.SelectionSet{
				Selections: []ast.Selection{
					ast.NewField(&ast.Field{
						Name: ast.NewName(&ast.Name{
							Value: "spec",
						}),
					}),
					ast.NewField(&ast.Field{
						Name: ast.NewName(&ast.Name{
							Value: "sensitive",
						}),
					}),
					ast.NewField(&ast.Field{
						Name: ast.NewName(&ast.Name{
							Value: "mask",
						}),
					}),
					ast.NewField(&ast.Field{
						Name: ast.NewName(&ast.Name{
							Value: "errors",
						}),
					}),
				},
			})

			valuekeys := ast.NewListValue(&ast.ListValue{})
			for _, name := range keys {
				valuekeys.Values = append(valuekeys.Values, ast.NewStringValue(&ast.StringValue{
					Value: name,
				}))
			}

			prevSelSet.Selections = append(prevSelSet.Selections,
				ast.NewField(&ast.Field{
					Name: ast.NewName(&ast.Name{
						Value: spec,
					}),
					Arguments: []*ast.Argument{
						ast.NewArgument(&ast.Argument{
							Name: ast.NewName(&ast.Name{
								Value: "insecure",
							}),
							Value: ast.NewVariable(&ast.Variable{
								Name: ast.NewName(&ast.Name{
									Value: "insecure",
								}),
							}),
						}),
						ast.NewArgument(&ast.Argument{
							Name: ast.NewName(&ast.Name{
								Value: "keys",
							}),
							Value: valuekeys,
						}),
					},
					SelectionSet: nextSelSet,
				}))

			return nextSelSet
		}

		topSelSet := ast.NewSelectionSet(&ast.SelectionSet{})
		nextSelSet := topSelSet

		// todo: poor Sebastian's deduplication
		slices.Sort(specKeys)
		prev := ""
		for _, specKey := range specKeys {
			if prev == specKey {
				continue
			}
			prev = specKey
			nextSelSet = nextVarSpecs(varSpecs, specKey, nextSelSet)
		}

		doneSelSet := ast.NewSelectionSet(&ast.SelectionSet{})
		nextSelSet.Selections = append(nextSelSet.Selections, ast.NewField(&ast.Field{
			Name: ast.NewName(&ast.Name{
				Value: "done",
			}),
			SelectionSet: doneSelSet,
		}))

		selSet.Selections = append(selSet.Selections,
			ast.NewField(&ast.Field{
				Name: ast.NewName(&ast.Name{
					Value: "validate",
				}),
				// Arguments: []*ast.Argument{
				// 	ast.NewArgument(&ast.Argument{
				// 		Name: ast.NewName(&ast.Name{
				// 			Value: "insecure",
				// 		}),
				// 		Value: ast.NewVariable(&ast.Variable{
				// 			Name: ast.NewName(&ast.Name{
				// 				Value: "insecure",
				// 			}),
				// 		}),
				// 	}),
				// },
				SelectionSet: topSelSet,
			}),
		)

		return doneSelSet, nil
	}
}

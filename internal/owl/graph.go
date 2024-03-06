package owl

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"

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

func specResolver(mutator func(*setVar)) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		insecure := p.Args["insecure"].(bool)
		keysArg := p.Args["keys"].([]interface{})

		opSet := p.Source.(*OperationSet)
		for _, kArg := range keysArg {
			if insecure {
				continue
			}
			k := kArg.(string)
			v := opSet.items[k]
			mutator(v)
		}

		return p.Source, nil
	}
}

func init() {
	SpecTypes = make(map[string]*specType)

	sensitiveResolver := specResolver(func(v *setVar) {
		v.Value.Status = "MASKED"
		if len(v.Value.Resolved) > 24 {
			v.Value.Resolved = v.Value.Resolved[:3] + "..." + v.Value.Resolved[len(v.Value.Resolved)-3:]
			return
		}
		v.Value.Resolved = ""
	})

	SpecTypes["Secret"] = registerSpec("Secret", true, true, sensitiveResolver)
	SpecTypes["Password"] = registerSpec("Password", true, true, sensitiveResolver)
	SpecTypes["Opaque"] = registerSpec("Opaque", true, false,
		specResolver(func(v *setVar) {
			v.Value.Status = "HIDDEN"
			v.Value.Original = v.Value.Resolved
			v.Value.Resolved = ""
		}),
	)
	SpecTypes["Plain"] = registerSpec("Plain", false, false,
		specResolver(func(v *setVar) {
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
				"key": &graphql.Field{
					Type: graphql.String,
				},
				// "raw": &graphql.Field{
				// 	Type: graphql.String,
				// },
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
							"checked": &graphql.Field{
								Type: graphql.Boolean,
							},
						},
					}),
				},
				"required": &graphql.Field{
					Type: graphql.Boolean,
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
				"created": &graphql.Field{
					Type: graphql.DateTime,
				},
				"updated": &graphql.Field{
					Type: graphql.DateTime,
				},
			},
		},
	)

	VariableInputType := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "VariableInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"key": &graphql.InputObjectFieldConfig{
				Type: graphql.String,
			},
			"raw": &graphql.InputObjectFieldConfig{
				Type: graphql.String,
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
						"checked": &graphql.InputObjectFieldConfig{
							Type:         graphql.Boolean,
							DefaultValue: false,
						},
					},
				}),
			},
			"required": &graphql.InputObjectFieldConfig{
				Type:         graphql.Boolean,
				DefaultValue: false,
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
			"created": &graphql.InputObjectFieldConfig{
				Type: graphql.DateTime,
			},
			"updated": &graphql.InputObjectFieldConfig{
				Type: graphql.DateTime,
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
					},
					Resolve: resolveOperation(resolveLoadOrUpdate),
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
					},
					Resolve: resolveOperation(resolveLoadOrUpdate),
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
						snapshot := SetVarResult{}
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

						for _, v := range opSet.items {
							snapshot = append(snapshot, v)
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

func resolveOperation(resolveMutator func(SetVarResult, *OperationSet, bool) error) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		vars, ok := p.Args["vars"]
		if !ok {
			return p.Source, nil
		}
		hasSpecs := p.Args["hasSpecs"].(bool)

		var resolverOpSet *OperationSet
		var err error

		switch p.Source.(type) {
		case *OperationSet:
			resolverOpSet = p.Source.(*OperationSet)
		default:
			resolverOpSet, err = NewOperationSet(WithOperation(TransientSetOperation, "resolver"))
			if err != nil {
				return nil, err
			}
		}

		buf, err := json.Marshal(vars)
		if err != nil {
			return nil, err
		}

		var revive SetVarResult
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

func reduceSetOperations(store *Store, vars io.StringWriter) QueryNodeReducer {
	return func(opDef *ast.OperationDefinition, selSet *ast.SelectionSet) (*ast.SelectionSet, error) {
		opSetData := make(map[string]SetVarResult, len(store.opSets))

		for i, opSet := range store.opSets {
			if len(opSet.items) == 0 {
				continue
			}

			opName := ""
			switch opSet.operation.kind {
			case LoadSetOperation:
				opName = "load"
			case UpdateSetOperation:
				opName = "update"
			case DeleteSetOperation:
				opName = "delete"
			default:
				continue
			}
			nvars := fmt.Sprintf("%s_%d", opName, i)

			for _, v := range opSet.items {
				opSetData[nvars] = append(opSetData[nvars], v)
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
			nextSelSet.Selections = append(nextSelSet.Selections, ast.NewField(&ast.Field{
				Name: ast.NewName(&ast.Name{
					Value: "location",
				}),
			}))
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
						Value: "key",
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
						},
					}),
				}),
				ast.NewField(&ast.Field{
					Name: ast.NewName(&ast.Name{
						Value: "required",
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

func reduceSepcs(store *Store) QueryNodeReducer {
	return func(opDef *ast.OperationDefinition, selSet *ast.SelectionSet) (*ast.SelectionSet, error) {
		var specKeys []string
		varSpecs := make(map[string]*setVar)
		for _, opSet := range store.opSets {
			if len(opSet.items) == 0 {
				continue
			}
			for _, v := range opSet.items {
				if _, ok := SpecTypes[v.Spec.Name]; !ok {
					return nil, fmt.Errorf("unknown spec type: %s on %s", v.Spec.Name, v.Key)
				}
				varSpecs[v.Key] = v
				specKeys = append(specKeys, v.Spec.Name)
			}
		}

		nextVarSpecs := func(varSpecs map[string]*setVar, spec string, prevSelSet *ast.SelectionSet) *ast.SelectionSet {
			var keys []string
			for _, v := range varSpecs {
				if v.Spec.Name != spec {
					continue
				}
				keys = append(keys, v.Key)
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

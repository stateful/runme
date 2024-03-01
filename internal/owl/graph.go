package owl

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/printer"
)

var (
	Schema     graphql.Schema
	Operations map[setOperationKind]string
)

func init() {
	var VariableType = graphql.NewObject(
		graphql.ObjectConfig{
			Name: "Variable",
			Fields: graphql.Fields{
				"key": &graphql.Field{
					Type: graphql.String,
				},
				// "raw": &graphql.Field{
				// 	Type: graphql.String,
				// },
				"value": &graphql.Field{
					Type: graphql.NewObject(graphql.ObjectConfig{
						Name: "VariableValue",
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
						Name: "VariableSpec",
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
						Name: "VariableOperation",
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

	var EnvironmentType *graphql.Object
	EnvironmentType = graphql.NewObject(graphql.ObjectConfig{
		Name: "Environment",
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
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						vars, ok := p.Args["vars"]
						if !ok {
							return p.Source, nil
						}
						hasSpecs := p.Args["hasSpecs"].(bool)

						var flatOpSet *operationSet
						var err error

						switch p.Source.(type) {
						case *operationSet:
							flatOpSet = p.Source.(*operationSet)
						default:
							flatOpSet, err = NewOperationSet(WithOperation(SnapshotSetOperation, "query"))
							if err != nil {
								return nil, err
							}
						}

						buf, err := json.Marshal(vars)
						if err != nil {
							return nil, err
						}

						var revive []setVar
						err = json.Unmarshal(buf, &revive)
						if err != nil {
							return nil, err
						}

						for _, v := range revive {
							old, ok := flatOpSet.items[v.Key]
							if hasSpecs && ok {
								old.Spec = v.Spec
								old.Required = v.Required
								continue
							}
							if ok {
								v.Created = old.Created
							}
							v.Updated = v.Created
							flatOpSet.items[v.Key] = &v
						}

						return flatOpSet, nil
					},
				},
				"location": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						switch p.Source.(type) {
						case *operationSet:
							opSet := p.Source.(*operationSet)
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
						opSet, ok := p.Source.(*operationSet)
						if !ok {
							return nil, errors.New("source is not an OperationSet")
						}

						var snapshot setVarResult
						for _, v := range opSet.items {
							if !insecure {
								// todo: move "masking" into to "type system"
								switch v.Spec.Name {
								case "Plain", "Path":
									v.Value.Status = "LITERAL"
								case "Secret", "Password":
									v.Value.Status = "MASKED"
									if len(v.Value.Resolved) > 24 {
										v.Value.Resolved = v.Value.Resolved[:3] + "..." + v.Value.Resolved[len(v.Value.Resolved)-3:]
										break
									}
									v.Value.Resolved = ""
								default:
									v.Value.Status = "HIDDEN"
									v.Value.Original = v.Value.Resolved
									v.Value.Resolved = ""
								}
							}

							snapshot = append(snapshot, v)
						}
						snapshot.sort()

						return snapshot, nil
					},
				},
				// "renderVars": newRender(),
			}
		}),
	})

	var err error
	Schema, err = graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(
			graphql.ObjectConfig{Name: "Query",
				Fields: graphql.Fields{
					"environment": &graphql.Field{
						Type: EnvironmentType,
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

func (s *Store) addLoadVarsNode(opDef *ast.OperationDefinition, vars io.StringWriter) (*ast.SelectionSet, error) {
	selSet := ast.NewSelectionSet(&ast.SelectionSet{})
	opDef.SelectionSet.Selections[0].(*ast.Field).SelectionSet = selSet
	opSetData := make(map[string]setVarResult, len(s.opSets))

	for i, opSet := range s.opSets {
		nvars := fmt.Sprintf("load_%d", i)

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
				Value: "load",
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

	opSetJson, err := json.MarshalIndent(opSetData, "", " ")
	if err != nil {
		return nil, err
	}
	vars.WriteString(string(opSetJson))

	return selSet, nil
}

func (s *Store) snapshotQuery(query, vars io.StringWriter) error {
	opDef := ast.NewOperationDefinition(&ast.OperationDefinition{
		Operation: "query",
		Name: ast.NewName(&ast.Name{
			Value: "ResolveEnvSnapshot",
		}),
		Directives: []*ast.Directive{},
		SelectionSet: ast.NewSelectionSet(&ast.SelectionSet{
			Selections: []ast.Selection{
				ast.NewField(&ast.Field{
					Name: ast.NewName(&ast.Name{
						Value: "environment",
					}),
					Arguments:  []*ast.Argument{},
					Directives: []*ast.Directive{},
				}),
			},
		}),
		VariableDefinitions: []*ast.VariableDefinition{
			ast.NewVariableDefinition(&ast.VariableDefinition{
				Variable: ast.NewVariable(&ast.Variable{
					Name: ast.NewName(&ast.Name{
						Value: "insecure",
					}),
				}),
				Type: ast.NewNamed(&ast.Named{
					Name: ast.NewName(&ast.Name{
						Value: "Boolean",
					}),
				}),
				DefaultValue: ast.NewBooleanValue(&ast.BooleanValue{
					Value: false,
				}),
			}),
		},
	})

	selSet, err := s.addLoadVarsNode(opDef, vars)
	if err != nil {
		return err
	}

	_, err = addSnapshotQueryNode(selSet)
	if err != nil {
		return err
	}

	doc := ast.NewDocument(&ast.Document{
		Definitions: []ast.Node{opDef},
	})
	res := printer.Print(doc)

	text, ok := res.(string)
	if !ok {
		return errors.New("ast printer returned unknown type")
	}
	query.WriteString(text)

	return nil
}

func addSnapshotQueryNode(selSet *ast.SelectionSet) (*ast.SelectionSet, error) {
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

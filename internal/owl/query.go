package owl

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"time"

	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/printer"
	"go.uber.org/zap"
)

func (s *Store) snapshotQuery(query, vars io.StringWriter) error {
	varDefs := []*ast.VariableDefinition{
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
	}

	loaded, updated, deleted := 0, 0, 0
	for _, opSet := range s.opSets {
		if len(opSet.specs) == 0 && len(opSet.values) == 0 {
			continue
		}
		switch opSet.operation.kind {
		case LoadSetOperation:
			loaded++
		case UpdateSetOperation:
			updated++
		case DeleteSetOperation:
			deleted++
		}

	}
	s.logger.Debug("snapshot opSets breakdown", zap.Int("loaded", loaded), zap.Int("updated", updated), zap.Int("deleted", deleted))

	q, err := NewQuery("Snapshot", varDefs,
		[]QueryNodeReducer{
			reconcileAsymmetry(s),
			reduceSetOperations(s, vars),
			reduceSepcs(s),
			reduceSnapshot(),
		},
	)
	if err != nil {
		return err
	}

	text, err := q.Print()
	if err != nil {
		return err
	}

	_, err = query.WriteString(text)
	if err != nil {
		return err
	}

	return nil
}

func reduceSetOperations(store *Store, vars io.StringWriter) QueryNodeReducer {
	return func(opDef *ast.OperationDefinition, selSet *ast.SelectionSet) (*ast.SelectionSet, error) {
		opSetData := make(map[string]SetVarItems, len(store.opSets))

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
					// ast.NewArgument(&ast.Argument{
					// 	Name: ast.NewName(&ast.Name{
					// 		Value: "location",
					// 	}),
					// 	Value: ast.NewStringValue(&ast.StringValue{
					// 		Value: opSet.operation.location,
					// 	}),
					// }),
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
									Value: "origin",
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
												Value: "source",
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
					Value: "render",
				}),
				SelectionSet: ast.NewSelectionSet(&ast.SelectionSet{
					Selections: []ast.Selection{
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
					},
				}),
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

		deltaOpSet, err := NewOperationSet(WithOperation(ReconcileSetOperation))
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

type Query struct {
	doc *ast.Document
}

func NewQuery(name string, varDefs []*ast.VariableDefinition, reducers []QueryNodeReducer) (*Query, error) {
	selSet := ast.NewSelectionSet(&ast.SelectionSet{})
	opDef := ast.NewOperationDefinition(&ast.OperationDefinition{
		Operation: "query",
		Name: ast.NewName(&ast.Name{
			Value: fmt.Sprintf("ResolveOwl%s", name),
		}),
		Directives: []*ast.Directive{},
		SelectionSet: ast.NewSelectionSet(&ast.SelectionSet{
			Selections: []ast.Selection{
				ast.NewField(&ast.Field{
					Name: ast.NewName(&ast.Name{
						Value: "environment",
					}),
					Arguments:    []*ast.Argument{},
					Directives:   []*ast.Directive{},
					SelectionSet: selSet,
				}),
			},
		}),
		VariableDefinitions: varDefs,
	})

	var err error
	for _, reducer := range reducers {
		if selSet, err = reducer(opDef, selSet); err != nil {
			return nil, err
		}
	}

	doc := ast.NewDocument(&ast.Document{Definitions: []ast.Node{opDef}})

	return &Query{doc: doc}, nil
}

func (q *Query) Print() (string, error) {
	res := printer.Print(q.doc)
	text, ok := res.(string)
	if !ok {
		return "", errors.New("ast printer returned unknown type")
	}

	return text, nil
}

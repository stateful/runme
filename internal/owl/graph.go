package owl

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/printer"
)

type setOperationKind int

const (
	LoadSetOperation setOperationKind = iota
	UpdateSetOperation
	DeleteSetOperation
	SnapshotSetOperation
)

type Operation struct {
	kind     setOperationKind
	location string
}

type operationSet struct {
	operation Operation
	hasSpecs  bool
	items     map[string]*setVar
}

type setVarOperation struct {
	Order    uint             `json:"order"`
	Kind     setOperationKind `json:"kind"`
	Location string           `json:"location"`
}

type setVarValue struct {
	// Type    string `json:"type"`
	Literal string `json:"literal"`
	// Success bool   `json:"success"`
	// ValidationErrors validator.ValidationErrors `json:"-"`
}

type setVarSpec struct {
	Name    string `json:"name"`
	Checked bool   `json:"checked"`
}

type setVar struct {
	Key       string           `json:"key"`
	Raw       string           `json:"raw"`
	Value     *setVarValue     `json:"value,omitempty"`
	Spec      *setVarSpec      `json:"spec,omitempty"`
	Required  bool             `json:"required"`
	Operation *setVarOperation `json:"operation"`
	Created   *time.Time       `json:"created,omitempty"`
	Updated   *time.Time       `json:"updated,omitempty"`
}

type operationSetOption func(*operationSet) error

func NewOperationSet(opts ...operationSetOption) (*operationSet, error) {
	opSet := &operationSet{
		hasSpecs: false,
		items:    make(map[string]*setVar),
	}

	for _, opt := range opts {
		if err := opt(opSet); err != nil {
			return nil, err
		}
	}
	return opSet, nil
}

func WithOperation(operation setOperationKind, location string) operationSetOption {
	return func(opSet *operationSet) error {
		opSet.operation = Operation{
			kind:     operation,
			location: location,
		}
		return nil
	}
}

func WithSpecs(included bool) operationSetOption {
	return func(opSet *operationSet) error {
		opSet.hasSpecs = included
		return nil
	}
}

func (s *operationSet) addEnvs(envs ...string) (err error) {
	for _, env := range envs {
		err = s.addRaw([]byte(env))
	}
	return err
}

func (s *operationSet) addRaw(raw []byte) error {
	lines := bytes.Split(raw, []byte{'\n'})
	for _, rawLine := range lines {
		line := bytes.Trim(rawLine, " \r")
		if len(line) > 0 && line[0] == '#' {
			continue
		}
		if len(bytes.Trim(line, " \r\n")) <= 0 {
			continue
		}

		created := time.Now()
		k, val, spec, required := ParseRawSpec(line)
		if len(spec) == 0 {
			spec = "Opaque"
		}
		s.items[k] = &setVar{
			Key:      k,
			Raw:      string(line),
			Value:    &setVarValue{Literal: val},
			Spec:     &setVarSpec{Name: spec, Checked: false},
			Required: required,
			Created:  &created,
		}
	}
	return nil
}

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
							"literal": &graphql.Field{
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
						"literal": &graphql.InputObjectFieldConfig{
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

						var opSet *operationSet
						var err error

						switch p.Source.(type) {
						case *operationSet:
							opSet = p.Source.(*operationSet)
						default:
							opSet, err = NewOperationSet(WithOperation(SnapshotSetOperation, "query"))
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
							old, ok := opSet.items[v.Key]
							if hasSpecs {
								if !ok {
									break
								}
								old.Spec = v.Spec
								continue
							}
							if ok {
								v.Created = old.Created
								continue
							}
							v.Updated = v.Created
							opSet.items[v.Key] = &v
						}

						return opSet, nil
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
						opSet, ok := p.Source.(*operationSet)
						if !ok {
							return nil, errors.New("source is not an OperationSet")
						}

						var snapshot []*setVar
						for _, v := range opSet.items {
							snapshot = append(snapshot, v)
						}

						slices.SortFunc(snapshot, func(i, j *setVar) int {
							if i.Spec.Name != j.Spec.Name {
								return strings.Compare(i.Spec.Name, j.Spec.Name)
							}
							return strings.Compare(i.Key, j.Key)
						})

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
	var opsVarDefs []*ast.VariableDefinition
	opSetData := make(map[string][]*setVar, len(s.opSets))
	for i, opSet := range s.opSets {
		nvars := fmt.Sprintf("load_%d", i)

		for _, v := range opSet.items {
			opSetData[nvars] = append(opSetData[nvars], v)
		}

		opsVarDefs = append(opsVarDefs, ast.NewVariableDefinition(&ast.VariableDefinition{
			Variable: ast.NewVariable(&ast.Variable{
				Name: ast.NewName(&ast.Name{
					Value: nvars,
				}),
			}),
			Type: ast.NewNamed(&ast.Named{
				Name: ast.NewName(&ast.Name{
					Value: "[VariableInput]",
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

	opDef.VariableDefinitions = opsVarDefs
	opSetJson, err := json.MarshalIndent(opSetData, "", " ")
	if err != nil {
		return nil, err
	}
	vars.WriteString(string(opSetJson))

	return selSet, nil
}

func (s *Store) snapshotQuery(query, vars io.StringWriter) error {
	opsDef := ast.NewOperationDefinition(&ast.OperationDefinition{
		Operation: "query",
		Name: ast.NewName(&ast.Name{
			Value: "ResolveEnv",
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
	})

	selSet, err := s.addLoadVarsNode(opsDef, vars)
	if err != nil {
		return err
	}

	_, err = addSnapshotQueryNode(selSet)
	if err != nil {
		return err
	}

	doc := ast.NewDocument(&ast.Document{
		Definitions: []ast.Node{opsDef},
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
								Value: "literal",
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
					Value: ast.NewBooleanValue(&ast.BooleanValue{
						Value: false,
					}),
				}),
				// ast.NewArgument(&ast.Argument{
				// 	Name: ast.NewName(&ast.Name{
				// 		Value: "specsOnly",
				// 	}),
				// 	Value: ast.NewBooleanValue(&ast.BooleanValue{
				// 		Value: false,
				// 	}),
				// }),
			},
			SelectionSet: nextSelSet,
		}),
	)
	return nextSelSet, nil
}

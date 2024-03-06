package owl

import (
	"errors"
	"fmt"

	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/printer"
)

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

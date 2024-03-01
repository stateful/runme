package owl

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/graphql-go/graphql"
)

type Store struct {
	// mu     sync.RWMutex
	opSets []*OperationSet
}

type StoreOption func(*Store) error

func NewStore(opts ...StoreOption) (*Store, error) {
	s := &Store{}

	for _, opt := range opts {
		if err := opt(s); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func WithSpecFile(specFile string, raw []byte) StoreOption {
	return func(s *Store) error {
		opSet := NewOperationSet(LoadSetOperation, specFile)
		err := opSet.addRaw(raw)
		if err != nil {
			return err
		}

		s.opSets = append(s.opSets, opSet)
		return nil
	}
}

func WithEnvs(envs ...string) StoreOption {
	return func(s *Store) error {
		opSet := NewOperationSet(LoadSetOperation, "process")
		err := opSet.addEnvs(envs...)
		if err != nil {
			return err
		}

		s.opSets = append(s.opSets, opSet)
		return nil
	}
}

func (s *Store) Snapshot() error {
	var query, vars bytes.Buffer
	err := s.snapshotQuery(&query, &vars)
	if err != nil {
		return err
	}

	fmt.Println(query.String())

	var varValues map[string]interface{}
	err = json.Unmarshal(vars.Bytes(), &varValues)
	if err != nil {
		return err
	}

	result := graphql.Do(graphql.Params{
		Schema:         Schema,
		RequestString:  query.String(),
		VariableValues: varValues,
		RootObject:     varValues,
	})

	if result.HasErrors() {
		return fmt.Errorf("%v", result.Errors)
	}

	fmt.Printf("%+v\n", result.Data)

	return nil
}

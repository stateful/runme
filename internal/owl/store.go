package owl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/graphql-go/graphql"
	"gopkg.in/yaml.v3"
)

type Store struct {
	// mu     sync.RWMutex
	opSets []*operationSet
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
		opSet, err := NewOperationSet(WithOperation(LoadSetOperation, specFile), WithSpecs(true))
		if err != nil {
			return err
		}

		err = opSet.addRaw(raw)
		if err != nil {
			return err
		}

		s.opSets = append(s.opSets, opSet)
		return nil
	}
}

func WithEnvs(envs ...string) StoreOption {
	return func(s *Store) error {
		opSet, err := NewOperationSet(WithOperation(LoadSetOperation, "process"), WithSpecs(false))
		if err != nil {
			return err
		}

		err = opSet.addEnvs(envs...)
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

	fmt.Fprintf(os.Stderr, "%s", query.String())

	var varValues map[string]interface{}
	err = json.Unmarshal(vars.Bytes(), &varValues)
	if err != nil {
		return err
	}

	result := graphql.Do(graphql.Params{
		Schema:         Schema,
		RequestString:  query.String(),
		VariableValues: varValues,
	})

	if result.HasErrors() {
		return fmt.Errorf("graphql errors %s", result.Errors)
	}

	rYaml, _ := yaml.Marshal(result.Data)
	fmt.Println(string(rYaml))

	return nil
}

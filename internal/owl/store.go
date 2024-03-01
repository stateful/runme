package owl

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/graphql-go/graphql"
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
	return withSpecsFile(specFile, raw, true)
}

func WithEnvFile(specFile string, raw []byte) StoreOption {
	return withSpecsFile(specFile, raw, false)
}

func withSpecsFile(specFile string, raw []byte, hasSpecs bool) StoreOption {
	return func(s *Store) error {
		opSet, err := NewOperationSet(WithOperation(LoadSetOperation, specFile), WithSpecs(hasSpecs))
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

func (s *Store) Snapshot() ([]*setVar, error) {
	var query, vars bytes.Buffer
	err := s.snapshotQuery(&query, &vars)
	if err != nil {
		return nil, err
	}

	fmt.Fprintf(os.Stderr, "%s", query.String())

	var varValues map[string]interface{}
	err = json.Unmarshal(vars.Bytes(), &varValues)
	if err != nil {
		return nil, err
	}

	result := graphql.Do(graphql.Params{
		Schema:         Schema,
		RequestString:  query.String(),
		VariableValues: varValues,
	})

	if result.HasErrors() {
		return nil, fmt.Errorf("graphql errors %s", result.Errors)
	}

	val, err := traverse(result.Data, "snapshot")
	if err != nil {
		return nil, err
	}

	j, err := json.MarshalIndent(val, "", " ")
	if err != nil {
		return nil, err
	}

	fmt.Println(string(j))

	var snapshot []*setVar
	_ = json.Unmarshal(j, &snapshot)

	return snapshot, nil
}

func traverse(data interface{}, segment string) (interface{}, error) {
	m, ok := data.(map[string]interface{})
	if !ok {
		return nil, errors.New("not a map")
	}
	for k, v := range m {
		if k == segment {
			return v, nil
		}
		switch v.(type) {
		case map[string]interface{}:
			return traverse(v, segment)
		}
	}
	return nil, nil
}

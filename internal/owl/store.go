package owl

import (
	"bytes"
	"os"
	"sync"
	"time"
)

type Store struct {
	mu     sync.RWMutex
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

func WithSpecFile(filePath string) StoreOption {
	return func(s *Store) error {
		s.mu.Lock()
		defer s.mu.Unlock()

		raw, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}

		opSet := NewOperationSet(LoadStoreOperation)
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
		s.mu.Lock()
		defer s.mu.Unlock()

		opSet := NewOperationSet(LoadStoreOperation)
		err := opSet.addEnvs(envs...)
		if err != nil {
			return err
		}

		s.opSets = append(s.opSets, opSet)
		return nil
	}
}

type SetOperation int

const (
	LoadStoreOperation SetOperation = iota
	UpdateStoreOperation
	DeleteStoreOperation
)

type OperationSet struct {
	operation SetOperation
	items     map[string]setVar
}

type varValue struct {
	// Type    string `json:"type"`
	Literal string `json:"literal"`
	// Success bool   `json:"success"`
	// ValidationErrors validator.ValidationErrors `json:"-"`
}

type varSpec struct {
	Name    string `json:"name"`
	Checked bool   `json:"checked"`
}

type varOrigin struct {
	Order    uint   `json:"order"`
	Type     string `json:"type"`
	Location string `json:"location"`
}

type setVar struct {
	Key       string     `json:"key"`
	Raw       string     `json:"raw"`
	Value     *varValue  `json:"value,omitempty"`
	Spec      *varSpec   `json:"spec,omitempty"`
	Origin    *varOrigin `json:"origin"`
	Mandatory bool       `json:"mandatory"`
	Created   *time.Time `json:"created,omitempty"`
	Updated   *time.Time `json:"updated,omitempty"`
}

func NewOperationSet(operation SetOperation) *OperationSet {
	return &OperationSet{
		operation: operation,
		items:     make(map[string]setVar),
	}
}

func (s *OperationSet) addEnvs(envs ...string) (err error) {
	for _, env := range envs {
		err = s.addRaw([]byte(env))
	}
	return err
}

func (s *OperationSet) addRaw(raw []byte) error {
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
		k, val, spec, mandatory := parseRawSpec(line)
		if len(spec) == 0 {
			spec = "Opaque"
		}
		s.items[k] = setVar{
			Key:       k,
			Raw:       string(line),
			Value:     &varValue{Literal: val},
			Spec:      &varSpec{Name: spec, Checked: false},
			Mandatory: mandatory,
			Created:   &created,
		}
	}
	return nil
}

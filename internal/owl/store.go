package owl

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
	"go.uber.org/zap"
)

type setOperationKind int

const (
	LoadSetOperation setOperationKind = iota
	UpdateSetOperation
	DeleteSetOperation
	TransientSetOperation
)

type Operation struct {
	kind     setOperationKind
	location string
}

type OperationSet struct {
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
	Original string `json:"original,omitempty"`
	Resolved string `json:"resolved,omitempty"`
	Status   string `json:"status"`
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

type SetVarResult []*setVar

func (res SetVarResult) sortbyKey() {
	slices.SortStableFunc(res, func(i, j *setVar) int {
		return strings.Compare(i.Key, j.Key)
	})
}

func (res SetVarResult) sort() {
	slices.SortStableFunc(res, func(i, j *setVar) int {
		if i.Spec.Name != "Opaque" && j.Spec.Name != "Opaque" {
			return 0
		}
		if i.Spec.Name != "Opaque" {
			return -1
		}
		if j.Spec.Name != "Opaque" {
			return 1
		}
		return strings.Compare(i.Key, j.Key)
	})
}

type OperationSetOption func(*OperationSet) error

func NewOperationSet(opts ...OperationSetOption) (*OperationSet, error) {
	opSet := &OperationSet{
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

func WithOperation(operation setOperationKind, location string) OperationSetOption {
	return func(opSet *OperationSet) error {
		opSet.operation = Operation{
			kind:     operation,
			location: location,
		}
		return nil
	}
}

func WithSpecs(included bool) OperationSetOption {
	return func(opSet *OperationSet) error {
		opSet.hasSpecs = included
		return nil
	}
}

func (s *OperationSet) addEnvs(envs ...string) (err error) {
	for _, env := range envs {
		if len(strings.Split(env, "=")) == 1 {
			env = env + "="
		}
		err = s.addRaw([]byte(env), false)
	}
	return err
}

func (s *OperationSet) addRaw(raw []byte, fromFile bool) error {
	lines := [][]byte{raw}

	if fromFile {
		lines = bytes.Split(raw, []byte{'\n'})
	}

	for _, rawLine := range lines {
		line := rawLine
		if fromFile {
			line := bytes.Trim(rawLine, " \r")
			if len(line) > 0 && line[0] == '#' {
				continue
			}
			if len(bytes.Trim(line, " \r\n")) <= 0 {
				continue
			}
		}

		created := time.Now()
		k, val, spec, required := ParseRawSpec(line)
		if len(spec) == 0 {
			spec = "Opaque"
		}
		s.items[k] = &setVar{
			Key:      k,
			Raw:      string(line),
			Value:    &setVarValue{Resolved: val},
			Spec:     &setVarSpec{Name: spec, Checked: false},
			Required: required,
			Created:  &created,
		}
	}

	return nil
}

func resolveLoadOrUpdate(vars SetVarResult, resolverOpSet *OperationSet, hasSpecs bool) error {
	for _, v := range vars {
		old, ok := resolverOpSet.items[v.Key]
		if hasSpecs && ok {
			old.Spec = v.Spec
			old.Required = v.Required
			continue
		}
		if ok {
			v.Created = old.Created
		}
		v.Updated = v.Created
		resolverOpSet.items[v.Key] = v
	}
	return nil
}

func resolveDelete(vars SetVarResult, resolverOpSet *OperationSet, _ bool) error {
	for _, v := range vars {
		_, ok := resolverOpSet.items[v.Key]
		if !ok {
			continue
		}
		delete(resolverOpSet.items, v.Key)
	}
	return nil
}

type Store struct {
	mu     sync.RWMutex
	opSets []*OperationSet

	logger *zap.Logger
}

type StoreOption func(*Store) error

func NewStore(opts ...StoreOption) (*Store, error) {
	s := &Store{logger: zap.NewNop()}

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

		err = opSet.addRaw(raw, true)
		if err != nil {
			return err
		}

		s.opSets = append(s.opSets, opSet)
		return nil
	}
}

func WithEnvs(envs ...string) StoreOption {
	return func(s *Store) error {
		opSet, err := NewOperationSet(WithOperation(LoadSetOperation, "session"), WithSpecs(false))
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

func WithLogger(logger *zap.Logger) StoreOption {
	return func(s *Store) error {
		s.logger = logger
		return nil
	}
}

func (s *Store) InsecureValues() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items, err := s.snapshot(true)
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, len(items))
	for _, item := range items {
		result = append(result, item.Key+"="+item.Value.Resolved)
	}

	return result, nil
}

func (s *Store) Update(newOrUpdated, deleted []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	updateOpSet, err := NewOperationSet(WithOperation(UpdateSetOperation, "exec"), WithSpecs(false))
	if err != nil {
		return err
	}

	err = updateOpSet.addEnvs(newOrUpdated...)
	if err != nil {
		return err
	}

	s.opSets = append(s.opSets, updateOpSet)

	deleteOpSet, err := NewOperationSet(WithOperation(DeleteSetOperation, "exec"), WithSpecs(false))
	if err != nil {
		return err
	}

	err = deleteOpSet.addEnvs(deleted...)
	if err != nil {
		return err
	}

	s.opSets = append(s.opSets, deleteOpSet)

	return nil
}

func (s *Store) validateQuery(query, vars io.StringWriter) error {
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

	q, err := NewQuery("Validate", varDefs,
		[]QueryNodeReducer{
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

func (s *Store) snapshot(insecure bool) (SetVarResult, error) {
	var query, vars bytes.Buffer
	err := s.snapshotQuery(&query, &vars)
	if err != nil {
		return nil, err
	}

	// s.logger.Debug("snapshot query", zap.String("query", query.String()))
	// fmt.Println(query.String())

	var varValues map[string]interface{}
	err = json.Unmarshal(vars.Bytes(), &varValues)
	if err != nil {
		return nil, err
	}
	varValues["insecure"] = insecure

	// j, err := json.MarshalIndent(varValues, "", " ")
	// if err != nil {
	// 	return nil, err
	// }
	// s.logger.Debug("snapshot vars", zap.String("vars", string(j)))

	result := graphql.Do(graphql.Params{
		Schema:         Schema,
		RequestString:  query.String(),
		VariableValues: varValues,
	})

	if result.HasErrors() {
		return nil, fmt.Errorf("graphql errors %s", result.Errors)
	}

	val, err := extractDataKey(result.Data, "snapshot")
	if err != nil {
		return nil, err
	}

	j, err := json.MarshalIndent(val, "", " ")
	if err != nil {
		return nil, err
	}

	var snapshot SetVarResult
	_ = json.Unmarshal(j, &snapshot)

	return snapshot, nil
}

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
		if len(opSet.items) == 0 {
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

func extractDataKey(data interface{}, key string) (interface{}, error) {
	m, ok := data.(map[string]interface{})
	if !ok {
		return nil, errors.New("not a map")
	}
	var found interface{}
	var err error
	for k, v := range m {
		if k == key {
			return v, nil
		}
		switch v.(type) {
		case map[string]interface{}:
			found, err = extractDataKey(v, key)
			if err == nil {
				break
			}
		default:
			continue
		}
		if found != nil {
			break
		}
	}
	return found, nil
}

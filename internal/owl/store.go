package owl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/graphql-go/graphql"
	"github.com/stateful/godotenv"
	commandpkg "github.com/stateful/runme/v3/internal/command"
	"go.uber.org/zap"
)

type setOperationKind int

const (
	LoadSetOperation setOperationKind = iota
	UpdateSetOperation
	DeleteSetOperation
	ReconcileSetOperation
	TransientSetOperation
)

type Operation struct {
	kind setOperationKind
	// location string
}

type OperationSet struct {
	operation Operation
	hasSpecs  bool
	specs     map[string]*SetVarSpec
	values    map[string]*SetVarValue
}

type ComplexOperationSet struct {
	*OperationSet
	Name      string
	Namespace string
	Keys      []string
}

type setVarOperation struct {
	Order  uint             `json:"-"`
	Kind   setOperationKind `json:"kind"`
	Source string           `json:"source"`
}

type varValue struct {
	Original  string           `json:"original,omitempty"`
	Resolved  string           `json:"resolved,omitempty"`
	Status    string           `json:"status,omitempty"`
	Operation *setVarOperation `json:"operation"`
}

type varSpec struct {
	Name        string           `json:"name"`
	Required    bool             `json:"required"`
	Description string           `json:"description"`
	Complex     string           `json:"-"`
	Namespace   string           `json:"-"`
	Rules       string           `json:"validator,omitempty"`
	Operation   *setVarOperation `json:"operation"`
	Error       ValidationError  `json:"-"`
	Checked     bool             `json:"checked"`
}

type SetVar struct {
	Key    string `json:"key"`
	Origin string `json:"origin,omitempty"`
	// Operation *setVarOperation `json:"operation"`
	Created *time.Time `json:"created,omitempty"`
	Updated *time.Time `json:"updated,omitempty"`
}

type SetVarSpec struct {
	Var  *SetVar  `json:"var,omitempty"`
	Spec *varSpec `json:"spec,omitempty"`
}

type SetVarValue struct {
	Var   *SetVar   `json:"var,omitempty"`
	Value *varValue `json:"value,omitempty"`
}

type SetVarError struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type SetVarItem struct {
	Var    *SetVar        `json:"var,omitempty"`
	Value  *varValue      `json:"value,omitempty"`
	Spec   *varSpec       `json:"spec,omitempty"`
	Errors []*SetVarError `json:"errors,omitempty"`
}

type SetVarItems []*SetVarItem

func (res SetVarItems) sortbyKey() {
	slices.SortStableFunc(res, func(i, j *SetVarItem) int {
		return strings.Compare(i.Var.Key, j.Var.Key)
	})
}

func (res SetVarItems) getSensitiveKeys(data interface{}) ([]string, error) {
	validate, err := extractDataKey(data, "validate")
	if err != nil {
		return nil, errors.Wrap(err, "failed to extract validate key")
	}

	m, ok := validate.(map[string]interface{})
	if !ok {
		return nil, errors.New("not a map")
	}

	specsSensitivity := make(map[string]bool)
	var recurse func(map[string]interface{}, string)
	recurse = func(m map[string]interface{}, parentName string) {
		_, found := specsSensitivity[parentName]
		if !found && parentName != "" {
			specsSensitivity[parentName] = true
		}
		for k, v := range m {
			// truly sensitive secrets require both mask & sensitive to be true
			if k == "mask" || k == "sensitive" {
				specsSensitivity[parentName] = specsSensitivity[parentName] && v.(bool)
			}

			if k == "done" {
				break
			}

			n, ok := v.(map[string]interface{})
			if !ok {
				continue
			}

			recurse(n, k)
		}
	}
	recurse(m, "")

	var filtered []string
	for _, item := range res {
		if specsSensitivity[item.Spec.Name] {
			filtered = append(filtered, item.Var.Key)
		}
	}

	return filtered, nil
}

func (res SetVarItems) sort() {
	slices.SortFunc(res, func(i, j *SetVarItem) int {
		if i.Spec == nil {
			return -1
		}
		if j.Spec == nil {
			return 1
		}
		if i.Spec.Name != "Opaque" && j.Spec.Name != "Opaque" {
			jUpdated := j.Var.Updated.Unix()
			iUpdated := i.Var.Updated.Unix()

			delta := int(jUpdated - iUpdated)

			if delta == 0 {
				return strings.Compare(i.Var.Key, j.Var.Key)
			}
			return delta
		}
		if i.Spec.Name != "Opaque" {
			return -1
		}
		if j.Spec.Name != "Opaque" {
			return 1
		}
		return strings.Compare(i.Var.Key, j.Var.Key)
	})
}

type OperationSetOption func(*OperationSet) error

func NewOperationSet(opts ...OperationSetOption) (*OperationSet, error) {
	opSet := &OperationSet{
		hasSpecs: false,
		specs:    make(map[string]*SetVarSpec),
		values:   make(map[string]*SetVarValue),
	}

	for _, opt := range opts {
		if err := opt(opSet); err != nil {
			return nil, err
		}
	}
	return opSet, nil
}

func WithOperation(operation setOperationKind) OperationSetOption {
	return func(opSet *OperationSet) error {
		opSet.operation = Operation{
			kind: operation,
			// location: location,
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

func WithItems(items SetVarItems) OperationSetOption {
	return func(opSet *OperationSet) error {
		for _, item := range items {
			if item.Spec != nil {
				opSet.specs[item.Var.Key] = &SetVarSpec{Var: item.Var, Spec: item.Spec}
			}
			if item.Value != nil {
				opSet.values[item.Var.Key] = &SetVarValue{Var: item.Var, Value: item.Value}
			}
		}
		return nil
	}
}

func (s *OperationSet) addEnvs(source string, envs ...string) error {
	for _, env := range envs {
		parts := strings.Split(env, "=")
		k, v := parts[0], ""
		if len(parts) > 1 {
			v = strings.Join(parts[1:], "=")
		}

		created := time.Now()
		s.values[k] = &SetVarValue{
			Var: &SetVar{
				Key:     k,
				Created: &created,
				// Operation: &setVarOperation{Source: source},
			},
			Value: &varValue{
				Original:  v,
				Operation: &setVarOperation{Source: source},
			},
		}
	}
	return nil
}

func (s *OperationSet) addRaw(raw []byte, source string, hasSpecs bool) error {
	vals, comments, err := godotenv.UnmarshalBytesWithComments(raw)
	if err != nil {
		return err
	}

	specs := ParseRawSpec(vals, comments)
	for key, spec := range specs {
		created := time.Now()

		switch hasSpecs {
		case true:
			s.specs[key] = &SetVarSpec{
				Var: &SetVar{
					Key: key,
					// Operation: &setVarOperation{Source: source},
					Created: &created,
				},
				Spec: &varSpec{
					Name:        string(spec.Name),
					Required:    spec.Required,
					Description: vals[key],
					Operation:   &setVarOperation{Source: source},
					Checked:     false,
				},
			}
		default:
			s.values[key] = &SetVarValue{
				Var: &SetVar{
					Key:     key,
					Created: &created,
				},
				Value: &varValue{
					Original: vals[key],
					Status:   "UNRESOLVED",
				},
			}
		}

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
		opSet, err := NewOperationSet(WithOperation(LoadSetOperation), WithSpecs(hasSpecs))
		if err != nil {
			return err
		}

		err = opSet.addRaw(raw, specFile, hasSpecs)
		if err != nil {
			return err
		}

		s.opSets = append(s.opSets, opSet)
		return nil
	}
}

func WithEnvs(source string, envs ...string) StoreOption {
	return func(s *Store) error {
		opSet, err := NewOperationSet(WithOperation(LoadSetOperation), WithSpecs(false))
		if err != nil {
			return err
		}

		err = opSet.addEnvs(source, envs...)
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

func (s *Store) Snapshot() (SetVarItems, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items, err := s.snapshot(false)
	if err != nil {
		return nil, err
	}

	return items, nil
}

func (s *Store) InsecureGet(k string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var query, vars bytes.Buffer
	err := s.getterQuery(&query, &vars)
	if err != nil {
		return "", err
	}

	// s.logger.Debug("getter query", zap.String("query", query.String()))
	// _, _ = fmt.Println(query.String())

	var varValues map[string]interface{}
	err = json.Unmarshal(vars.Bytes(), &varValues)
	if err != nil {
		return "", err
	}
	varValues["key"] = k
	varValues["insecure"] = true

	// j, err := json.Marshal(varValues)
	// if err != nil {
	// 	return "", err
	// }
	// fmt.Println(string(j))
	// s.logger.Debug("insecure getter", zap.String("vars", string(j)))

	result := graphql.Do(graphql.Params{
		Schema:         Schema,
		RequestString:  query.String(),
		VariableValues: varValues,
	})

	if result.HasErrors() {
		return "", fmt.Errorf("graphql errors %s", result.Errors)
	}

	val, err := extractDataKey(result.Data, "get")
	if err != nil {
		return "", err
	}

	j, err := json.Marshal(val)
	if err != nil {
		return "", err
	}

	var res *SetVarItem
	err = json.Unmarshal(j, &res)
	if err != nil {
		return "", err
	}

	if res.Value == nil {
		return "", nil
	}

	return res.Value.Resolved, nil
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
		result = append(result, item.Var.Key+"="+item.Value.Resolved)
	}

	return result, nil
}

func (s *Store) SensitiveKeys() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var query, vars bytes.Buffer
	err := s.sensitiveKeysQuery(&query, &vars)
	if err != nil {
		return nil, err
	}

	// s.logger.Debug("sensitive keys query", zap.String("query", query.String()))
	// _, _ = fmt.Println(query.String())

	var varValues map[string]interface{}
	err = json.Unmarshal(vars.Bytes(), &varValues)
	if err != nil {
		return nil, err
	}

	// j, err := json.Marshal(varValues)
	// if err != nil {
	// 	return nil, err
	// }
	// fmt.Println(string(j))
	// s.logger.Debug("sensitiveKeys vars", zap.String("vars", string(j)))

	result := graphql.Do(graphql.Params{
		Schema:         Schema,
		RequestString:  query.String(),
		VariableValues: varValues,
	})

	if result.HasErrors() {
		return nil, fmt.Errorf("graphql errors %s", result.Errors)
	}

	val, err := extractDataKey(result.Data, "sensitiveKeys")
	if err != nil {
		return nil, err
	}

	j, err := json.Marshal(val)
	if err != nil {
		return nil, err
	}

	var keyResults SetVarItems
	err = json.Unmarshal(j, &keyResults)
	if err != nil {
		return nil, err
	}

	keyResults.sortbyKey()
	keys, err := keyResults.getSensitiveKeys(result.Data)
	if err != nil {
		return nil, err
	}

	return keys, nil
}

func (s *Store) Update(context context.Context, newOrUpdated, deleted []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	execInfo, ok := context.Value(commandpkg.ExecutionInfoKey).(*commandpkg.ExecutionInfo)
	if !ok {
		return errors.New("execution info not found in context")
	}

	execRef := fmt.Sprintf("#%s", execInfo.KnownID)
	if execInfo.KnownName != "" {
		execRef = fmt.Sprintf("#%s", execInfo.KnownName)
	}

	if len(newOrUpdated) > 0 {
		updateOpSet, err := NewOperationSet(WithOperation(UpdateSetOperation), WithSpecs(false))
		if err != nil {
			return err
		}

		err = updateOpSet.addEnvs(execRef, newOrUpdated...)
		if err != nil {
			return err
		}

		s.opSets = append(s.opSets, updateOpSet)
	}

	if len(deleted) > 0 {
		deleteOpSet, err := NewOperationSet(WithOperation(DeleteSetOperation), WithSpecs(false))
		if err != nil {
			return err
		}

		err = deleteOpSet.addEnvs(execRef, deleted...)
		if err != nil {
			return err
		}

		s.opSets = append(s.opSets, deleteOpSet)
	}

	// s.logger.Debug("opSets size", zap.Int("size", len(s.opSets)))

	return nil
}

func (s *Store) snapshot(insecure bool) (SetVarItems, error) {
	var query, vars bytes.Buffer
	err := s.snapshotQuery(&query, &vars)
	if err != nil {
		return nil, err
	}

	// s.logger.Debug("snapshot query", zap.String("query", query.String()))
	// _, _ = fmt.Println(query.String())

	var varValues map[string]interface{}
	err = json.Unmarshal(vars.Bytes(), &varValues)
	if err != nil {
		return nil, err
	}
	varValues["insecure"] = insecure

	// j, err := json.Marshal(varValues)
	// if err != nil {
	// 	return nil, err
	// }
	// fmt.Println(string(j))
	// s.logger.Debug("snapshot vars", zap.String("vars", string(j)))

	result := graphql.Do(graphql.Params{
		Schema:         Schema,
		RequestString:  query.String(),
		VariableValues: varValues,
	})

	if result.HasErrors() {
		return nil, fmt.Errorf("graphql errors %s", result.Errors)
	}

	// rj, _ := json.MarshalIndent(result.Data, "", " ")
	// fmt.Println(string(rj))

	val, err := extractDataKey(result.Data, "snapshot")
	if err != nil {
		return nil, err
	}

	j, err := json.Marshal(val)
	if err != nil {
		return nil, err
	}

	var snapshot SetVarItems
	_ = json.Unmarshal(j, &snapshot)

	return snapshot, nil
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

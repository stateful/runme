package runner

import (
	"errors"
	"strings"
	"sync"

	"golang.org/x/exp/slices"
)

// Limited by windows
const maxEnvSize = 32760

type envStore struct {
	values map[string]string
	mu     sync.RWMutex
}

func newEnvStore(envs ...string) *envStore {
	s := &envStore{values: make(map[string]string)}
	s.Add(envs...)
	return s
}

func (s *envStore) Add(envs ...string) *envStore {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, env := range envs {
		k, v := splitEnv(env)
		s.values[k] = v
	}
	return s
}

func (s *envStore) Get(k string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.values[k]
}

func getEnvSizeContribution(k, v string) int {
	// +2 for the '=' and '\0' separators
	return len(k) + len(v) + 2
}

func (s *envStore) Set(k string, v string) (*envStore, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	newSize := getEnvSizeContribution(k, v)
	for key, value := range s.values {
		if key == k {
			continue
		}

		newSize += getEnvSizeContribution(key, value)
	}

	if newSize > maxEnvSize {
		return s, errors.New("could not set environment variable, environment size limit exceeded")
	}

	s.values[k] = v
	return s, nil
}

func (s *envStore) Delete(envs ...string) *envStore {
	s.mu.Lock()
	defer s.mu.Unlock()

	temp := newEnvStore(envs...)
	for k := range temp.values {
		delete(s.values, k)
	}
	return s
}

func (s *envStore) Values() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]string, 0, len(s.values))
	for k, v := range s.values {
		result = append(result, k+"="+v)
	}
	slices.Sort(result)
	return result, nil
}

func diffEnvStores(store, updated *envStore) (newOrUpdated, unchanged, deleted []string) {
	for k, v := range store.values {
		uVal, ok := updated.values[k]
		if !ok {
			deleted = append(deleted, k)
		} else if v != uVal {
			newOrUpdated = append(newOrUpdated, k+"="+uVal)
		} else {
			unchanged = append(unchanged, k)
		}
	}
	for k, v := range updated.values {
		_, ok := store.values[k]
		if ok {
			continue
		}
		newOrUpdated = append(newOrUpdated, k+"="+v)
	}
	return
}

func splitEnv(str string) (string, string) {
	parts := strings.SplitN(str, "=", 2)
	switch len(parts) {
	case 0:
		return "", ""
	case 1:
		return parts[0], ""
	default:
		return parts[0], parts[1]
	}
}

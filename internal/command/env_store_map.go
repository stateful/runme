package command

import (
	"context"
	"slices"
	"sync"
)

type envStoreMap struct {
	mu    sync.RWMutex
	items map[string]string
}

func newEnvStore() *envStoreMap {
	return &envStoreMap{items: make(map[string]string)}
}

var _ EnvStore = new(envStoreMap)

func (s *envStoreMap) Load(source string, envs ...string) error {
	return s.Merge(context.Background(), envs...)
}

func (s *envStoreMap) Merge(ctx context.Context, envs ...string) error {
	for _, env := range envs {
		k, v := splitEnv(env)
		if err := s.Set(ctx, k, v); err != nil {
			return err
		}
	}
	return nil
}

func (s *envStoreMap) Get(k string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.items[k]
	return v, ok
}

func (s *envStoreMap) Set(ctx context.Context, k, v string) error {
	if len(k)+len(v) > MaxEnvSizeInBytes {
		return ErrEnvTooLarge
	}
	s.mu.Lock()
	s.items[k] = v
	s.mu.Unlock()
	return nil
}

func (s *envStoreMap) Delete(ctx context.Context, k string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, k)
	return nil
}

func (s *envStoreMap) Items() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]string, 0, len(s.items))
	for k, v := range s.items {
		result = append(result, k+"="+v)
	}
	slices.Sort(result)
	return result, nil
}

func diffEnvStores(initial, updated *envStoreMap) (newOrUpdated, unchanged, deleted []string) {
	initial.mu.RLock()
	defer initial.mu.RUnlock()
	updated.mu.RLock()
	defer updated.mu.RUnlock()

	for k, v := range initial.items {
		uVal, ok := updated.items[k]
		if !ok {
			deleted = append(deleted, k)
		} else if v != uVal {
			newOrUpdated = append(newOrUpdated, k+"="+uVal)
		} else {
			unchanged = append(unchanged, k)
		}
	}
	for k, v := range updated.items {
		_, ok := initial.items[k]
		if ok {
			continue
		}
		newOrUpdated = append(newOrUpdated, k+"="+v)
	}
	return
}

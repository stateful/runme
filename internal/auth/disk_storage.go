package auth

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sync"
)

type DiskStorage struct {
	// Location is a directory where data will be stored.
	Location string

	cleanupOnce sync.Once
}

var _ Storage = (*DiskStorage)(nil)

func (s *DiskStorage) Path(key string) string {
	return s.filePath(key)
}

func (s *DiskStorage) Load(key string, val interface{}) error {
	s.cleanupLegacy()

	f, err := os.Open(s.filePath(key))
	switch {
	case os.IsNotExist(err):
		return ErrNotFound
	case err != nil:
		return err
	}
	defer func() { _ = f.Close() }()

	data, err := io.ReadAll(io.LimitReader(f, 4*1024))
	if err != nil {
		return err
	}

	return json.Unmarshal(data, val)
}

func (s *DiskStorage) Save(key string, val interface{}) error {
	s.cleanupLegacy()

	if err := os.MkdirAll(s.Location, 0o700); err != nil {
		return err
	}

	data, err := json.Marshal(val)
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath(key), data, 0o600)
}

func (s *DiskStorage) Delete(key string) error {
	return os.Remove(s.filePath(key))
}

func (s *DiskStorage) filePath(key string) string {
	return filepath.Join(s.Location, key+".json")
}

func (s *DiskStorage) cleanupLegacy() {
	s.cleanupOnce.Do(func() {
		// Delete old token.json that stored Stateful JWT token.
		_ = os.Remove(s.filePath("token"))
	})
}

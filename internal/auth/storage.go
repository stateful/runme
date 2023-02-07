package auth

import (
	"errors"
)

type Storage interface {
	Path(string) string
	Load(string, interface{}) error
	Save(string, interface{}) error
	Delete(string) error
}

var ErrNotFound = errors.New("value not found")

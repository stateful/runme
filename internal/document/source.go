package document

import (
	"io/fs"

	"github.com/pkg/errors"
)

type Source struct {
	data []byte
}

func NewSource(data []byte) *Source {
	return &Source{data: data}
}

func NewSourceFromFile(f fs.FS, filename string) (*Source, error) {
	data, err := fs.ReadFile(f, filename)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return NewSource(data), nil
}

func (s *Source) Parse() *ParsedSource {
	return newDefaultParser().Parse(s.data)
}

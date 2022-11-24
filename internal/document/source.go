package document

import (
	"io/fs"

	"github.com/pkg/errors"
)

type Source struct {
	data     []byte
	renderer Renderer
}

func NewSource(data []byte, renderer Renderer) *Source {
	return &Source{data: data, renderer: renderer}
}

func NewSourceFromFile(f fs.FS, filename string, renderer Renderer) (*Source, error) {
	data, err := fs.ReadFile(f, filename)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return NewSource(data, renderer), nil
}

func (s *Source) Parse() *ParsedSource {
	return newDefaultParser().Parse(s.data, s.renderer)
}

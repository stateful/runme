package document

import (
	"bytes"
	"encoding/json"
	stderrors "errors"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	"github.com/runmedev/runme/v3/internal/ulid"
	"github.com/runmedev/runme/v3/internal/version"
)

var ErrFrontmatterInvalid = stderrors.New("invalid frontmatter")

const (
	frontmatterFormatYAML = "yaml"
	frontmatterFormatJSON = "json"
	frontmatterFormatTOML = "toml"
)

type formatter func(f *Frontmatter, reset bool) ([]byte, error)

var formatters = map[string]formatter{
	frontmatterFormatYAML: func(f *Frontmatter, reset bool) ([]byte, error) {
		m := make(map[string]interface{})
		unmarshalYaml := consumeFrontmatterSeparators(yaml.Unmarshal, false)
		if err := unmarshalYaml([]byte(f.raw), &m); err != nil {
			return nil, errors.WithStack(err)
		}

		switch {
		case reset:
			f.Runme = nil
			delete(m, "runme")
		case !f.Runme.IsEmpty():
			m["runme"] = f.Runme
		}

		var buf bytes.Buffer
		encoder := yaml.NewEncoder(&buf)
		encoder.SetIndent(2)
		if err := encoder.Encode(m); err != nil {
			return nil, errors.WithStack(err)
		}
		if err := encoder.Close(); err != nil {
			return nil, errors.WithStack(err)
		}
		if buf.Len() < 5 && strings.Trim(buf.String(), "\r\n") == "{}" {
			return nil, nil
		}
		return append(append([]byte("---\n"), buf.Bytes()...), []byte("---")...), nil
	},
	frontmatterFormatJSON: func(f *Frontmatter, reset bool) ([]byte, error) {
		m := make(map[string]interface{})
		unmarshalJSON := consumeFrontmatterSeparators(json.Unmarshal, false)
		if err := unmarshalJSON([]byte(f.raw), &m); err != nil {
			return nil, errors.WithStack(err)
		}

		switch {
		case reset:
			f.Runme = nil
			delete(m, "runme")
		case !f.Runme.IsEmpty():
			m["runme"] = f.Runme
		}

		data, err := json.MarshalIndent(m, "", "  ")
		if err != nil {
			return nil, errors.WithStack(err)
		}
		if len(data) < 5 && bytes.Equal(bytes.Trim(data, "\r\n"), []byte("{}")) {
			return nil, nil
		}
		return data, nil
	},
	frontmatterFormatTOML: func(f *Frontmatter, reset bool) ([]byte, error) {
		m := make(map[string]interface{})
		unmarshalTOML := consumeFrontmatterSeparators(toml.Unmarshal, false)
		if err := unmarshalTOML([]byte(f.raw), &m); err != nil {
			return nil, errors.WithStack(err)
		}

		switch {
		case reset:
			f.Runme = nil
			delete(m, "runme")
		case !f.Runme.IsEmpty():
			m["runme"] = f.Runme
		}

		data, err := toml.Marshal(m)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		if len(data) == 0 {
			return nil, nil
		}
		return append(append([]byte("+++\n"), data...), []byte("+++")...), nil
	},
}

type RunmeMetadataDocument struct {
	RelativePath string `yaml:"relativePath,omitempty" json:"relativePath,omitempty" toml:"relativePath,omitempty"`
}

type RunmeMetadataSession struct {
	ID      string `yaml:"id,omitempty" json:"id,omitempty" toml:"id,omitempty"`
	Updated string `yaml:"updated,omitempty" json:"updated,omitempty" toml:"updated,omitempty"`
}

func (s *RunmeMetadataSession) GetID() string {
	if s == nil {
		return ""
	}

	return s.ID
}

type RunmeMetadata struct {
	ID       string                 `yaml:"id,omitempty" json:"id,omitempty" toml:"id,omitempty"`
	Version  string                 `yaml:"version,omitempty" json:"version,omitempty" toml:"version,omitempty"`
	Document *RunmeMetadataDocument `yaml:"document,omitempty" json:"document,omitempty" toml:"document,omitempty"`
	Session  *RunmeMetadataSession  `yaml:"session,omitempty" json:"session,omitempty" toml:"session,omitempty"`
}

func (m *RunmeMetadata) IsEmpty() bool {
	if m == nil {
		return true
	}

	return m.ID == "" && m.Version == "" && m.Document == nil && m.Session == nil
}

type Frontmatter struct {
	Runme *RunmeMetadata `yaml:"runme,omitempty"`
	Shell string         `yaml:"shell"`
	Cwd   string         `yaml:"cwd"`
	// Deprecated Category in favor of Tag
	Category     string `yaml:"category"`
	Tag          string `yaml:"tag"`
	TerminalRows string `yaml:"terminalRows"`
	SkipPrompts  bool   `yaml:"skipPrompts,omitempty"`

	format string
	raw    string // using string to be able to compare using ==
}

func newBlankFrontmatter(format string) *Frontmatter {
	return &Frontmatter{
		format: format,
	}
}

func NewYAMLFrontmatter() *Frontmatter {
	f := newBlankFrontmatter(frontmatterFormatYAML)
	f.raw = ""
	return f
}

func newFrontmatter() *Frontmatter {
	return &Frontmatter{
		Runme: &RunmeMetadata{
			ID:      ulid.GenerateID(),
			Version: version.BaseVersion(),
		},

		format: frontmatterFormatYAML,
	}
}

func (f *Frontmatter) ResetRunme(requireIdentity bool) (*Frontmatter, error) {
	if f == nil {
		return nil, nil
	}

	if _, err := f.Marshal(requireIdentity); err != nil {
		return f, err
	}

	// remove runme frontmatter
	remove := !requireIdentity

	formatter := formatters[f.format]
	resetRaw, err := formatter(f, remove)
	if err != nil {
		return nil, err
	}

	// return nil if frontmatter is actually empty
	if resetRaw == nil {
		return nil, nil
	}
	f.raw = string(resetRaw)

	return f, nil
}

// Marshal returns a marshaled frontmatter including triple-dashed lines.
// If the identity is required, but Frontmatter is nil, a new one is created.
func (f *Frontmatter) Marshal(requireIdentity bool) ([]byte, error) {
	if f == nil {
		if !requireIdentity {
			return nil, nil
		}
		f = newFrontmatter()
	}
	return f.marshal(requireIdentity)
}

func (f *Frontmatter) marshal(requireIdentity bool) ([]byte, error) {
	if requireIdentity {
		f.ensureID()
	}

	formatter, ok := formatters[f.format]
	if !ok {
		panic("invariant: Frontmatter created with invalid format")
	}

	return formatter(f, false)
}

func (f *Frontmatter) ensureID() {
	if f.Runme.IsEmpty() {
		f.Runme = &RunmeMetadata{}
	}

	if !ulid.ValidID(f.Runme.ID) {
		f.Runme.ID = ulid.GenerateID()
	}

	if v, ok := version.BaseVersionAuthoritative(); ok {
		f.Runme.Version = v
	}
}

func (d *Document) Frontmatter() *Frontmatter {
	d.splitSource()

	d.parseFrontmatter()

	return d.frontmatter
}

func (d *Document) FrontmatterWithError() (*Frontmatter, error) {
	fmtr := d.Frontmatter()

	if d.splitSourceErr != nil {
		return nil, d.splitSourceErr
	}

	if d.parseFrontmatterErr != nil {
		return nil, d.parseFrontmatterErr
	}

	return fmtr, nil
}

func (d *Document) parseFrontmatter() {
	d.onceParseFrontmatter.Do(func() {
		d.frontmatter, d.parseFrontmatterErr = parseFrontmatter(d.frontmatterRaw)
	})
}

// TODO(adamb): it should be removed when the complete refactoring of the project is finished.
func ParseFrontmatter(raw []byte) (*Frontmatter, error) {
	return parseFrontmatter(raw)
}

type parserFunc func([]byte, any) error

func parseFrontmatter(raw []byte) (*Frontmatter, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	// We know that frontmatter is not empty,
	// so d.frontmatter won't be nil ever.
	// However, it may still be invalid and
	// this detail will be in d.parseFrontmatterErr.
	var f Frontmatter

	// raw, err := consumeFrontmatterSeparators(raw)
	// if err != nil {
	// 	return nil, err
	// }

	// TODO(adamb): discuss how to approach this in the most sensible way.
	// It can always return to the initial idea of returning all errors,
	// but the client will be left with the same problem.
	parsers := []parserFunc{
		consumeFrontmatterSeparators(yaml.Unmarshal, true),
		json.Unmarshal,
		consumeFrontmatterSeparators(json.Unmarshal, true),
		consumeFrontmatterSeparators(toml.Unmarshal, true),
	}
	parsersNames := []string{
		frontmatterFormatYAML,
		frontmatterFormatJSON,
		frontmatterFormatJSON,
		frontmatterFormatTOML,
	}
	errorsCount := 0

	var firstError error

	for idx, parser := range parsers {
		err := parser(raw, &f)
		if err == nil {
			f.format = parsersNames[idx]
			f.raw = string(raw)
			break
		}

		errorsCount++
		if firstError == nil {
			firstError = errors.Wrap(err, "failed to parse frontmatter content")
		}
	}

	// If all parsers returned errors, select the first one.
	if errorsCount == len(parsers) {
		return nil, firstError
	}

	return &f, nil
}

func consumeFrontmatterSeparators(parser parserFunc, strict bool) parserFunc {
	return func(raw []byte, f any) error {
		lines := bytes.Split(raw, []byte{'\n'})

		if len(lines) >= 2 && bytes.Equal(bytes.TrimSpace(lines[0]), bytes.TrimSpace(lines[len(lines)-1])) {
			raw = bytes.Join(lines[1:len(lines)-1], []byte{'\n'})
		} else if strict {
			return errors.WithStack(ErrFrontmatterInvalid)
		}

		return parser(raw, f)
	}
}

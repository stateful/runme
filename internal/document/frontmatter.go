package document

import (
	byteslib "bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/pelletier/go-toml/v2"
	parserv1 "github.com/stateful/runme/internal/gen/proto/go/runme/parser/v1"
	"github.com/stateful/runme/internal/identity"
	"github.com/stateful/runme/internal/version"
	"gopkg.in/yaml.v3"
)

type RunmeMetaData struct {
	ID      string `yaml:"id,omitempty" json:"id,omitempty" toml:"id,omitempty"`
	Version string `yaml:"version,omitempty" json:"version,omitempty" toml:"version,omitempty"`
}

type Frontmatter struct {
	Runme       RunmeMetaData `yaml:"runme,omitempty"`
	Shell       string        `yaml:"shell,omitempty"`
	Cwd         string        `yaml:"cwd,omitempty"`
	SkipPrompts bool          `yaml:"skipPrompts,omitempty"`
}

type FrontmatterParseInfo struct {
	yaml error
	json error
	toml error

	other error

	raw string
}

func NewFrontmatter() Frontmatter {
	return Frontmatter{
		Runme: RunmeMetaData{
			ID:      identity.GenerateID(),
			Version: version.BaseVersion(),
		},
	}
}

func (fpi *FrontmatterParseInfo) GetRaw() string {
	return fpi.raw
}

func (fpi FrontmatterParseInfo) YAMLError() error {
	return fpi.yaml
}

func (fpi FrontmatterParseInfo) JSONError() error {
	return fpi.json
}

func (fpi FrontmatterParseInfo) TOMLError() error {
	return fpi.toml
}

func (fpi FrontmatterParseInfo) Error() error {
	return fpi.other
}

func toJSONStr(f *Frontmatter, source []byte, requireIdentity bool) (string, error) {
	m := make(map[string]interface{})

	if err := json.Unmarshal(source, &m); err != nil {
		return "", err
	}

	if requireIdentity {
		f.ensureID()
		m["runme"] = f.Runme
	} else {
		delete(m, "runme")
	}

	dest, err := json.Marshal(m)
	if err != nil {
		return "", err
	}

	if len(m) == 0 {
		return "", nil
	}

	return fmt.Sprintf("---\n%s\n---", string(dest)), nil
}

func toYamlStr(f *Frontmatter, source []byte, requireIdentity bool) (string, error) {
	m := make(map[string]interface{})

	if err := yaml.Unmarshal(source, &m); err != nil {
		return "", err
	}

	if requireIdentity {
		f.ensureID()
		m["runme"] = f.Runme
	} else {
		delete(m, "runme")
	}

	var buf byteslib.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	err := encoder.Encode(&m)
	if err != nil {
		return "", err
	}

	source = buf.Bytes()
	err = encoder.Close()
	if err != nil {
		return "", err
	}

	if len(m) == 0 {
		return "", nil
	}

	return fmt.Sprintf("---\n%s---", string(source)), nil
}

func toTomlStr(f *Frontmatter, source []byte, requireIdentity bool) (string, error) {
	m := make(map[string]interface{})

	if err := toml.Unmarshal(source, &m); err != nil {
		return "", err
	}

	if requireIdentity {
		f.ensureID()
		m["runme"] = f.Runme
	} else {
		delete(m, "runme")
	}

	dest, err := toml.Marshal(m)
	if err != nil {
		return "", err
	}

	if len(m) == 0 {
		return "", nil
	}

	return fmt.Sprintf("+++\n%s+++", string(dest)), nil
}

// ParseFrontmatter extracts the Frontmatter from a raw string and identifies its format.
func ParseFrontmatter(raw string) (f Frontmatter, info FrontmatterParseInfo) {
	f, info = ParseFrontmatterWithIdentity(raw, true)
	return
}

func ParseFrontmatterWithIdentity(raw string, enabled bool) (f Frontmatter, info FrontmatterParseInfo) {
	lines := strings.Split(raw, "\n")

	if raw == "" {
		info.raw, info.yaml = toYamlStr(&f, []byte(raw), enabled)
		return
	}

	if len(lines) < 2 || strings.TrimSpace(lines[0]) != strings.TrimSpace(lines[len(lines)-1]) {
		info.other = errors.New("invalid frontmatter")
		return
	}

	raw = strings.Join(lines[1:len(lines)-1], "\n")

	bytes := []byte(raw)

	if info.yaml = yaml.Unmarshal(bytes, &f); info.yaml == nil {
		info.raw, info.yaml = toYamlStr(&f, bytes, enabled)
		return
	}

	if info.json = json.Unmarshal(bytes, &f); info.json == nil {
		info.raw, info.json = toJSONStr(&f, bytes, enabled)
		return
	}

	if info.toml = toml.Unmarshal(bytes, &f); info.toml == nil {
		info.raw, info.toml = toTomlStr(&f, bytes, enabled)
		return
	}

	info.raw, info.yaml = toYamlStr(&f, bytes, enabled)

	return
}

func (fmtr *Frontmatter) ensureID() {
	if !identity.ValidID(fmtr.Runme.ID) {
		fmtr.Runme.ID = identity.GenerateID()
	}

	fmtr.Runme.Version = version.BaseVersion()
}

func (fmtr Frontmatter) ToParser() *parserv1.Frontmatter {
	return &parserv1.Frontmatter{
		Shell:       fmtr.Shell,
		Cwd:         fmtr.Cwd,
		SkipPrompts: fmtr.SkipPrompts,
	}
}

// InjectFrontmatter injects a test id into a yaml document
func InjectFrontmatter(s string) string {
	format := `---
runme:
  id: %s
  version: "%s"
---

%s`

	return fmt.Sprintf(format, identity.GenerateID(), version.BaseVersion(), s)
}

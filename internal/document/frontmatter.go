package document

import (
	byteslib "bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/pelletier/go-toml/v2"
	parserv1 "github.com/stateful/runme/internal/gen/proto/go/runme/parser/v1"
	"github.com/stateful/runme/internal/idgen"
	"github.com/stateful/runme/internal/version"
	"gopkg.in/yaml.v3"
)

type RunmeMetaData struct {
	ID      string `protobuf:"bytes,1,opt,name=id,proto3" yaml:"id,omitempty" json:"id,omitempty" toml:"id,omitempty"`
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
}

func NewFrontmatter() Frontmatter {
	return Frontmatter{
		Runme: RunmeMetaData{
			ID:      idgen.GenerateID(),
			Version: version.BuildVersion,
		},
	}
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

// ParseFrontmatter extracts the Frontmatter from a raw string and identifies its format.
func ParseFrontmatter(raw string) (f Frontmatter, info FrontmatterParseInfo) {
	lines := strings.Split(raw, "\n")

	if len(lines) < 2 || strings.TrimSpace(lines[0]) != strings.TrimSpace(lines[len(lines)-1]) {
		info.other = errors.New("invalid frontmatter")
		return
	}

	raw = strings.Join(lines[1:len(lines)-1], "\n")

	bytes := []byte(raw)

	if info.json = json.Unmarshal(bytes, &f); info.json == nil {
		return
	}

	if info.yaml = yaml.Unmarshal(bytes, &f); info.yaml == nil {
		return
	}

	if info.toml = toml.Unmarshal(bytes, &f); info.toml == nil {
		return
	}

	return
}

// StringifyFrontmatter converts Frontmatter to a string based on the provided format.
func StringifyFrontmatter(raw string, f Frontmatter, info FrontmatterParseInfo) (result string) {
	fmMap := make(map[string]interface{})
	fmMap["runme"] = f.Runme

	lines := strings.Split(raw, "\n")

	if len(lines) < 2 || strings.TrimSpace(lines[0]) != strings.TrimSpace(lines[len(lines)-1]) {
		info.other = errors.New("invalid frontmatter 2")
		return
	}

	raw = strings.Join(lines[1:len(lines)-1], "\n")

	bytes := []byte(raw)

	switch {
	case info.json == nil:
		if err := json.Unmarshal(bytes, &fmMap); err != nil {
			return
		}

		bytes, _ := json.Marshal(fmMap)
		result = fmt.Sprintf("---\n%s\n---", string(bytes))
		return
	case info.yaml == nil:
		if err := yaml.Unmarshal(bytes, &fmMap); err != nil {
			return
		}

		var buf byteslib.Buffer
		encoder := yaml.NewEncoder(&buf)
		encoder.SetIndent(2)
		_ = encoder.Encode(&fmMap)
		bytes := buf.Bytes()
		_ = encoder.Close()

		result = fmt.Sprintf("---\n%s---", string(bytes))
		return
	case info.toml == nil:
		if err := toml.Unmarshal(bytes, &fmMap); err != nil {
			return
		}

		bytes, _ := toml.Marshal(fmMap)
		result = fmt.Sprintf("+++\n%s+++", string(bytes))
		return
	default:
		return ""
	}
}

func (fmtr *Frontmatter) EnsureID() {
	if !idgen.ValidID(fmtr.Runme.ID) {
		fmtr.Runme.ID = idgen.GenerateID()
	}

	if fmtr.Runme.Version == "" {
		fmtr.Runme.Version = version.BuildVersion
	}
}

func (fmtr Frontmatter) ToParser() *parserv1.Frontmatter {
	return &parserv1.Frontmatter{
		Runme: &parserv1.Runme{
			Id:      fmtr.Runme.ID,
			Version: fmtr.Runme.Version,
		},
		Shell:       fmtr.Shell,
		Cwd:         fmtr.Cwd,
		SkipPrompts: fmtr.SkipPrompts,
	}
}

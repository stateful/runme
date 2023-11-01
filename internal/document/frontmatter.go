package document

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/pelletier/go-toml/v2"
	parserv1 "github.com/stateful/runme/internal/gen/proto/go/runme/parser/v1"
	"github.com/stateful/runme/internal/idgen"
	"gopkg.in/yaml.v3"
)

const FrontmatterParsingVersion = "V2"

type RunmeMetaData struct {
	ID      string `protobuf:"bytes,1,opt,name=id,proto3"`
	Version string `yaml:"version,omitempty"`
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
			Version: FrontmatterParsingVersion,
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
func ParseFrontmatter(raw string) (Frontmatter, FrontmatterParseInfo) {
	var (
		f    Frontmatter
		info FrontmatterParseInfo
		err  error
	)

	// Split the input string by new lines.
	lines := strings.Split(raw, "\n")

	// Check for valid frontmatter delimiters.
	if len(lines) < 2 || strings.TrimSpace(lines[0]) != "---" || strings.TrimSpace(lines[len(lines)-1]) != "---" {
		info.other = errors.New("invalid frontmatter")
		return f, info
	}

	// Rejoin the lines to get the raw frontmatter content.
	raw = strings.Join(lines[1:len(lines)-1], "\n")

	// Attempt to unmarshal the frontmatter in various formats.
	if err = yaml.Unmarshal([]byte(raw), &f); err == nil {
		info.yaml = err
		return f, info
	}

	if err = json.Unmarshal([]byte(raw), &f); err == nil {
		info.json = err
		return f, info
	}

	if err = toml.Unmarshal([]byte(raw), &f); err == nil {
		info.toml = err
		return f, info
	}

	// If unmarshaling fails for all formats, record the error.
	info.other = err
	return f, info
}

// StringifyFrontmatter converts Frontmatter to a string based on the provided format.
func StringifyFrontmatter(f Frontmatter, info FrontmatterParseInfo) (result string) {
	var (
		bytes []byte
		err   error
	)

	switch {
	case info.yaml != nil:
		bytes, err = yaml.Marshal(f)
	case info.json != nil:
		bytes, err = json.Marshal(f)
	case info.toml != nil:
		bytes, err = toml.Marshal(f)
	default:
		bytes, err = yaml.Marshal(f)
	}

	if err == nil {
		result = fmt.Sprintf("---\n%s---", string(bytes))
	}

	return
}

func (fmtr *Frontmatter) EnsureID() {
	if !idgen.ValidID(fmtr.Runme.ID) {
		fmtr.Runme.ID = idgen.GenerateID()
	}

	if fmtr.Runme.Version == "" {
		fmtr.Runme.Version = FrontmatterParsingVersion
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

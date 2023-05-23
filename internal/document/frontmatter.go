package document

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

type Frontmatter struct {
	Shell string
}

type FrontmatterParseInfo struct {
	yaml error
	json error
	toml error

	other error
}

func (fpi FrontmatterParseInfo) YamlError() error {
	return fpi.yaml
}

func (fpi FrontmatterParseInfo) JsonError() error {
	return fpi.json
}

func (fpi FrontmatterParseInfo) TomlError() error {
	return fpi.toml
}

func (fpi FrontmatterParseInfo) Error() error {
	return fpi.other
}

func ParseFrontmatter(raw string) (f Frontmatter, info FrontmatterParseInfo) {
	lines := strings.Split(raw, "\n")

	if len(lines) < 1 || strings.TrimSpace(lines[0]) != strings.TrimSpace(lines[len(lines)-1]) {
		info.other = errors.New("invalid frontmatter")
		return
	}

	raw = strings.Join(lines[1:len(lines)-1], "\n")

	bytes := []byte(raw)

	if info.yaml = yaml.Unmarshal(bytes, &f); info.yaml == nil {
		return
	}

	if info.json = json.Unmarshal(bytes, &f); info.json == nil {
		return
	}

	if info.toml = toml.Unmarshal(bytes, &f); info.toml == nil {
		return
	}

	return
}

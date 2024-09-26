package config

import (
	_ "embed"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

//go:embed runme.default.yaml
var defaultRunmeYAML []byte

func init() {
	// Ensure the default configuration is valid.
	_, err := newDefault()
	if err != nil {
		panic(err)
	}
}

func newDefault() (*Config, error) {
	return ParseYAML(defaultRunmeYAML)
}

func Default() *Config {
	cfg, _ := newDefault()
	return cfg
}

// ParseYAML parses the given YAML items and returns a configuration object.
// Multiple items are merged into a single configuration. It uses a default
// configuration as a base.
func ParseYAML(items ...[]byte) (*Config, error) {
	items = append([][]byte{defaultRunmeYAML}, items...)
	return parseYAML(items...)
}

func parseYAML(items ...[]byte) (*Config, error) {
	version, err := parseVersionFromYAML(items[0])
	if err != nil {
		return nil, err
	}

	for i := 1; i < len(items); i++ {
		v, err := parseVersionFromYAML(items[i])
		if err != nil {
			return nil, err
		}
		if v != version {
			return nil, errors.Errorf("inconsistent versions: %s and %s", version, v)
		}
	}

	switch version {
	case "v1alpha1":
		config, err := parseAndMergeV1alpha1(items...)
		if err != nil {
			return nil, err
		}

		if err := validateConfig(config); err != nil {
			return nil, errors.Wrap(err, "failed to validate config")
		}

		return config, nil
	default:
		return nil, errors.Errorf("unknown version: %s", version)
	}
}

type versionOnly struct {
	Version string `yaml:"version"`
}

func parseVersionFromYAML(data []byte) (string, error) {
	var result versionOnly

	if err := yaml.Unmarshal(data, &result); err != nil {
		return "", errors.Wrap(err, "failed to unmarshal version")
	}

	return result.Version, nil
}

// parseAndMergeV1alpha1 parses items, which are raw YAML blobs,
// one-by-one into a single map. Then, marshals the map into raw JSON.
// Finally, unmarshals the JSON into a [Config] object.
// Double unmarshaling is required to take advantage of the
// auto-generated [Config.UnmarshalJSON] method which does
// validation.
func parseAndMergeV1alpha1(items ...[]byte) (*Config, error) {
	m := make(map[string]interface{})

	for _, data := range items {
		if err := yaml.Unmarshal(data, &m); err != nil {
			return nil, errors.Wrap(err, "failed to parse v1alpha1 config")
		}
	}

	flatten, err := json.Marshal(m)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse v1alpha1 config")
	}

	var config Config

	if err := json.Unmarshal(flatten, &config); err != nil {
		return nil, errors.Wrap(err, "failed to parse v1alpha1 config")
	}

	if err := validateConfig(&config); err != nil {
		return nil, errors.Wrap(err, "failed to validate v1alpha1 config")
	}

	return &config, nil
}

func validateConfig(cfg *Config) error {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	if err := validateInsideCwd(cfg.Project.Root, cwd); err != nil {
		return errors.Wrap(err, "project.root")
	}

	if err := validateInsideCwd(cfg.Project.Filename, cwd); err != nil {
		return errors.Wrap(err, "project.filename")
	}

	return nil
}

func validateInsideCwd(path, cwd string) error {
	rel, err := filepath.Rel(cwd, filepath.Join(cwd, path))
	if err != nil {
		return errors.WithStack(err)
	}
	if strings.HasPrefix(rel, "..") {
		return errors.New("outside of the current working directory")
	}
	return nil
}

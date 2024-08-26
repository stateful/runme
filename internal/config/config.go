package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/bufbuild/protovalidate-go"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"gopkg.in/yaml.v3"

	configv1alpha1 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/config/v1alpha1"
)

// Config is a flatten configuration of runme.yaml. The purpose of it is to
// unify all the different configuration versions into a single struct.
type Config struct {
	ProjectRoot             string
	ProjectFilename         string
	ProjectFindRepoUpward   bool
	ProjectIgnorePaths      []string
	ProjectDisableGitignore bool
	ProjectEnvUseSystemEnv  bool
	ProjectEnvSources       []string
	ProjectFilters          []*Filter

	RuntimeDockerEnabled         bool
	RuntimeDockerImage           string
	RuntimeDockerBuildContext    string
	RuntimeDockerBuildDockerfile string

	ServerAddress     string
	ServerTLSEnabled  bool
	ServerTLSCertFile string
	ServerTLSKeyFile  string

	LogEnabled bool
	LogPath    string
	LogVerbose bool
}

func (c *Config) Clone() *Config {
	clone := *c
	clone.ProjectIgnorePaths = make([]string, len(c.ProjectIgnorePaths))
	copy(clone.ProjectIgnorePaths, c.ProjectIgnorePaths)
	clone.ProjectEnvSources = make([]string, len(c.ProjectEnvSources))
	copy(clone.ProjectEnvSources, c.ProjectEnvSources)
	clone.ProjectFilters = make([]*Filter, len(c.ProjectFilters))
	for i, f := range c.ProjectFilters {
		clone.ProjectFilters[i] = &Filter{
			Type:      f.Type,
			Condition: f.Condition,
		}
	}
	return &clone
}

func Defaults() *Config {
	return defaults.Clone()
}

func ParseYAML(data []byte) (*Config, error) {
	version, err := parseVersionFromYAML(data)
	if err != nil {
		return nil, err
	}
	switch version {
	case "v1alpha1":
		cfg, err := parseYAMLv1alpha1(data)
		if err != nil {
			return nil, err
		}

		if err := validateProto(cfg); err != nil {
			return nil, errors.Wrap(err, "failed to validate v1alpha1 config")
		}

		config, err := configV1alpha1ToConfig(cfg)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert v1alpha1 config")
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

func parseYAMLv1alpha1(data []byte) (*configv1alpha1.Config, error) {
	mmap := make(map[string]any)

	if err := yaml.Unmarshal(data, &mmap); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal yaml")
	}

	delete(mmap, "version")

	// In order to properly handle JSON-related field options like `json_name`,
	// the YAML data is first marshaled to JSON and then unmarshaled to a proto message
	// using the protojson package.
	configJSONRaw, err := json.Marshal(mmap)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal yaml to json")
	}

	var cfg configv1alpha1.Config
	if err := protojson.Unmarshal(configJSONRaw, &cfg); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal json to proto")
	}
	return &cfg, nil
}

func configV1alpha1ToConfig(c *configv1alpha1.Config) (*Config, error) {
	project := c.GetProject()
	runtime := c.GetRuntime()
	server := c.GetServer()
	log := c.GetLog()

	var filters []*Filter
	for _, f := range c.GetProject().GetFilters() {
		filters = append(filters, &Filter{
			Type:      f.GetType().String(),
			Condition: f.GetCondition(),
		})
	}

	cfg := Defaults()
	cfg.ProjectRoot = project.GetRoot()
	cfg.ProjectFilename = project.GetFilename()
	setIfHasValue(&cfg.ProjectFindRepoUpward, project.GetFindRepoUpward())
	cfg.ProjectIgnorePaths = project.GetIgnorePaths()
	setIfHasValue(&cfg.ProjectDisableGitignore, project.GetDisableGitignore())
	setIfHasValue(&cfg.ProjectEnvUseSystemEnv, project.GetEnv().GetUseSystemEnv())
	cfg.ProjectEnvSources = project.GetEnv().GetSources()
	cfg.ProjectFilters = filters

	setIfHasValue(&cfg.RuntimeDockerEnabled, runtime.GetDocker().GetEnabled())
	cfg.RuntimeDockerImage = runtime.GetDocker().GetImage()
	cfg.RuntimeDockerBuildContext = runtime.GetDocker().GetBuild().GetContext()
	cfg.RuntimeDockerBuildDockerfile = runtime.GetDocker().GetBuild().GetDockerfile()

	cfg.ServerAddress = server.GetAddress()
	setIfHasValue(&cfg.ServerTLSEnabled, server.GetTls().GetEnabled())
	cfg.ServerTLSCertFile = server.GetTls().GetCertFile()
	cfg.ServerTLSKeyFile = server.GetTls().GetKeyFile()

	setIfHasValue(&cfg.LogEnabled, log.GetEnabled())
	cfg.LogPath = log.GetPath()
	setIfHasValue(&cfg.LogVerbose, log.GetVerbose())

	return cfg, nil
}

func setIfHasValue[T any](prop *T, val interface{ GetValue() T }) {
	if val != nil && !reflect.ValueOf(val).IsNil() {
		*prop = val.GetValue()
	}
}

func validateConfig(cfg *Config) error {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	if err := validateInsideCwd(cfg.ProjectRoot, cwd); err != nil {
		return errors.Wrap(err, "failed to validate project dir")
	}

	if err := validateInsideCwd(cfg.ProjectFilename, cwd); err != nil {
		return errors.Wrap(err, "failed to validate filename")
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

func validateProto(m protoreflect.ProtoMessage) error {
	v, err := protovalidate.New()
	if err != nil {
		return errors.WithStack(err)
	}
	return errors.WithStack(v.Validate(m))
}

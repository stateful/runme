// Code generated by github.com/atombender/go-jsonschema, DO NOT EDIT.

package config

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// Runme configuration schema
type Config struct {
	// Client corresponds to the JSON schema field "client".
	Client *ConfigClient `json:"client,omitempty" yaml:"client,omitempty"`

	// Log corresponds to the JSON schema field "log".
	Log *ConfigLog `json:"log,omitempty" yaml:"log,omitempty"`

	// Project corresponds to the JSON schema field "project".
	Project ConfigProject `json:"project" yaml:"project"`

	// Runtime corresponds to the JSON schema field "runtime".
	Runtime *ConfigRuntime `json:"runtime,omitempty" yaml:"runtime,omitempty"`

	// Server corresponds to the JSON schema field "server".
	Server *ConfigServer `json:"server,omitempty" yaml:"server,omitempty"`

	// Version corresponds to the JSON schema field "version".
	Version string `json:"version" yaml:"version"`
}

type ConfigClient struct {
	// ServerAddress corresponds to the JSON schema field "server_address".
	ServerAddress string `json:"server_address" yaml:"server_address"`
}

// UnmarshalJSON implements json.Unmarshaler.
func (j *ConfigClient) UnmarshalJSON(b []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	if _, ok := raw["server_address"]; raw != nil && !ok {
		return fmt.Errorf("field server_address in ConfigClient: required")
	}
	type Plain ConfigClient
	var plain Plain
	if err := json.Unmarshal(b, &plain); err != nil {
		return err
	}
	*j = ConfigClient(plain)
	return nil
}

type ConfigLog struct {
	// Enabled corresponds to the JSON schema field "enabled".
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Path corresponds to the JSON schema field "path".
	Path string `json:"path" yaml:"path"`

	// Verbose corresponds to the JSON schema field "verbose".
	Verbose bool `json:"verbose,omitempty" yaml:"verbose,omitempty"`
}

// UnmarshalJSON implements json.Unmarshaler.
func (j *ConfigLog) UnmarshalJSON(b []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	type Plain ConfigLog
	var plain Plain
	if err := json.Unmarshal(b, &plain); err != nil {
		return err
	}
	if v, ok := raw["enabled"]; !ok || v == nil {
		plain.Enabled = false
	}
	if v, ok := raw["path"]; !ok || v == nil {
		plain.Path = ""
	}
	if v, ok := raw["verbose"]; !ok || v == nil {
		plain.Verbose = false
	}
	*j = ConfigLog(plain)
	return nil
}

type ConfigProject struct {
	// DisableGitignore corresponds to the JSON schema field "disable_gitignore".
	DisableGitignore bool `json:"disable_gitignore,omitempty" yaml:"disable_gitignore,omitempty"`

	// Env corresponds to the JSON schema field "env".
	Env *ConfigProjectEnv `json:"env,omitempty" yaml:"env,omitempty"`

	// Filename corresponds to the JSON schema field "filename".
	Filename string `json:"filename,omitempty" yaml:"filename,omitempty"`

	// Filters corresponds to the JSON schema field "filters".
	Filters []ConfigProjectFiltersElem `json:"filters,omitempty" yaml:"filters,omitempty"`

	// FindRepoUpward corresponds to the JSON schema field "find_repo_upward".
	FindRepoUpward bool `json:"find_repo_upward,omitempty" yaml:"find_repo_upward,omitempty"`

	// Ignore corresponds to the JSON schema field "ignore".
	Ignore []string `json:"ignore,omitempty" yaml:"ignore,omitempty"`

	// Root corresponds to the JSON schema field "root".
	Root string `json:"root,omitempty" yaml:"root,omitempty"`
}

type ConfigProjectEnv struct {
	// Sources corresponds to the JSON schema field "sources".
	Sources []string `json:"sources,omitempty" yaml:"sources,omitempty"`

	// UseSystemEnv corresponds to the JSON schema field "use_system_env".
	UseSystemEnv bool `json:"use_system_env,omitempty" yaml:"use_system_env,omitempty"`
}

// UnmarshalJSON implements json.Unmarshaler.
func (j *ConfigProjectEnv) UnmarshalJSON(b []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	type Plain ConfigProjectEnv
	var plain Plain
	if err := json.Unmarshal(b, &plain); err != nil {
		return err
	}
	if v, ok := raw["use_system_env"]; !ok || v == nil {
		plain.UseSystemEnv = false
	}
	*j = ConfigProjectEnv(plain)
	return nil
}

type ConfigProjectFiltersElem struct {
	// Condition corresponds to the JSON schema field "condition".
	Condition string `json:"condition" yaml:"condition"`

	// Extra corresponds to the JSON schema field "extra".
	Extra ConfigProjectFiltersElemExtra `json:"extra,omitempty" yaml:"extra,omitempty"`

	// Type corresponds to the JSON schema field "type".
	Type ConfigProjectFiltersElemType `json:"type" yaml:"type"`
}

type ConfigProjectFiltersElemExtra map[string]interface{}

type ConfigProjectFiltersElemType string

const ConfigProjectFiltersElemTypeFILTERTYPEBLOCK ConfigProjectFiltersElemType = "FILTER_TYPE_BLOCK"
const ConfigProjectFiltersElemTypeFILTERTYPEDOCUMENT ConfigProjectFiltersElemType = "FILTER_TYPE_DOCUMENT"

var enumValues_ConfigProjectFiltersElemType = []interface{}{
	"FILTER_TYPE_BLOCK",
	"FILTER_TYPE_DOCUMENT",
}

// UnmarshalJSON implements json.Unmarshaler.
func (j *ConfigProjectFiltersElemType) UnmarshalJSON(b []byte) error {
	var v string
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	var ok bool
	for _, expected := range enumValues_ConfigProjectFiltersElemType {
		if reflect.DeepEqual(v, expected) {
			ok = true
			break
		}
	}
	if !ok {
		return fmt.Errorf("invalid value (expected one of %#v): %#v", enumValues_ConfigProjectFiltersElemType, v)
	}
	*j = ConfigProjectFiltersElemType(v)
	return nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (j *ConfigProjectFiltersElem) UnmarshalJSON(b []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	if _, ok := raw["condition"]; raw != nil && !ok {
		return fmt.Errorf("field condition in ConfigProjectFiltersElem: required")
	}
	if _, ok := raw["type"]; raw != nil && !ok {
		return fmt.Errorf("field type in ConfigProjectFiltersElem: required")
	}
	type Plain ConfigProjectFiltersElem
	var plain Plain
	if err := json.Unmarshal(b, &plain); err != nil {
		return err
	}
	*j = ConfigProjectFiltersElem(plain)
	return nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (j *ConfigProject) UnmarshalJSON(b []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	type Plain ConfigProject
	var plain Plain
	if err := json.Unmarshal(b, &plain); err != nil {
		return err
	}
	if v, ok := raw["disable_gitignore"]; !ok || v == nil {
		plain.DisableGitignore = false
	}
	if v, ok := raw["filename"]; !ok || v == nil {
		plain.Filename = ""
	}
	if v, ok := raw["find_repo_upward"]; !ok || v == nil {
		plain.FindRepoUpward = false
	}
	if v, ok := raw["root"]; !ok || v == nil {
		plain.Root = ""
	}
	*j = ConfigProject(plain)
	return nil
}

type ConfigRuntime struct {
	// Docker corresponds to the JSON schema field "docker".
	Docker *ConfigRuntimeDocker `json:"docker,omitempty" yaml:"docker,omitempty"`
}

type ConfigRuntimeDocker struct {
	// Build corresponds to the JSON schema field "build".
	Build *ConfigRuntimeDockerBuild `json:"build,omitempty" yaml:"build,omitempty"`

	// Enabled corresponds to the JSON schema field "enabled".
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Image corresponds to the JSON schema field "image".
	Image string `json:"image" yaml:"image"`
}

type ConfigRuntimeDockerBuild struct {
	// Context corresponds to the JSON schema field "context".
	Context string `json:"context" yaml:"context"`

	// Dockerfile corresponds to the JSON schema field "dockerfile".
	Dockerfile string `json:"dockerfile" yaml:"dockerfile"`
}

// UnmarshalJSON implements json.Unmarshaler.
func (j *ConfigRuntimeDockerBuild) UnmarshalJSON(b []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	if _, ok := raw["context"]; raw != nil && !ok {
		return fmt.Errorf("field context in ConfigRuntimeDockerBuild: required")
	}
	if _, ok := raw["dockerfile"]; raw != nil && !ok {
		return fmt.Errorf("field dockerfile in ConfigRuntimeDockerBuild: required")
	}
	type Plain ConfigRuntimeDockerBuild
	var plain Plain
	if err := json.Unmarshal(b, &plain); err != nil {
		return err
	}
	*j = ConfigRuntimeDockerBuild(plain)
	return nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (j *ConfigRuntimeDocker) UnmarshalJSON(b []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	if _, ok := raw["enabled"]; raw != nil && !ok {
		return fmt.Errorf("field enabled in ConfigRuntimeDocker: required")
	}
	if _, ok := raw["image"]; raw != nil && !ok {
		return fmt.Errorf("field image in ConfigRuntimeDocker: required")
	}
	type Plain ConfigRuntimeDocker
	var plain Plain
	if err := json.Unmarshal(b, &plain); err != nil {
		return err
	}
	*j = ConfigRuntimeDocker(plain)
	return nil
}

type ConfigServer struct {
	// Address corresponds to the JSON schema field "address".
	Address string `json:"address" yaml:"address"`

	// MaxMessageSize corresponds to the JSON schema field "max_message_size".
	MaxMessageSize int `json:"max_message_size,omitempty" yaml:"max_message_size,omitempty"`

	// Tls corresponds to the JSON schema field "tls".
	Tls *ConfigServerTls `json:"tls,omitempty" yaml:"tls,omitempty"`
}

type ConfigServerTls struct {
	// CertFile corresponds to the JSON schema field "cert_file".
	CertFile *string `json:"cert_file,omitempty" yaml:"cert_file,omitempty"`

	// Enabled corresponds to the JSON schema field "enabled".
	Enabled bool `json:"enabled" yaml:"enabled"`

	// KeyFile corresponds to the JSON schema field "key_file".
	KeyFile *string `json:"key_file,omitempty" yaml:"key_file,omitempty"`
}

// UnmarshalJSON implements json.Unmarshaler.
func (j *ConfigServerTls) UnmarshalJSON(b []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	if _, ok := raw["enabled"]; raw != nil && !ok {
		return fmt.Errorf("field enabled in ConfigServerTls: required")
	}
	type Plain ConfigServerTls
	var plain Plain
	if err := json.Unmarshal(b, &plain); err != nil {
		return err
	}
	*j = ConfigServerTls(plain)
	return nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (j *ConfigServer) UnmarshalJSON(b []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	if _, ok := raw["address"]; raw != nil && !ok {
		return fmt.Errorf("field address in ConfigServer: required")
	}
	type Plain ConfigServer
	var plain Plain
	if err := json.Unmarshal(b, &plain); err != nil {
		return err
	}
	if v, ok := raw["max_message_size"]; !ok || v == nil {
		plain.MaxMessageSize = 33554432.0
	}
	*j = ConfigServer(plain)
	return nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (j *Config) UnmarshalJSON(b []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	if _, ok := raw["project"]; raw != nil && !ok {
		return fmt.Errorf("field project in Config: required")
	}
	if _, ok := raw["version"]; raw != nil && !ok {
		return fmt.Errorf("field version in Config: required")
	}
	type Plain Config
	var plain Plain
	if err := json.Unmarshal(b, &plain); err != nil {
		return err
	}
	*j = Config(plain)
	return nil
}

package config

// StaticOIDCConfig replaces OIDC discovery with pinned, deployment-owned keys.
type StaticOIDCConfig struct {
	Issuer                  string          `json:"issuer" yaml:"issuer"`
	Audience                string          `json:"audience" yaml:"audience"`
	MaxTokenLifetimeSeconds int             `json:"maxTokenLifetimeSeconds" yaml:"maxTokenLifetimeSeconds"`
	ClockSkewSeconds        int             `json:"clockSkewSeconds" yaml:"clockSkewSeconds"`
	Keys                    []StaticOIDCKey `json:"keys" yaml:"keys"`
}

// StaticOIDCKey binds one public key to a subject and an IAM principal. Claims
// supplied by the signer cannot choose a different authorized principal.
type StaticOIDCKey struct {
	Kid           string `json:"kid" yaml:"kid"`
	Algorithm     string `json:"algorithm" yaml:"algorithm"`
	PublicKeyFile string `json:"publicKeyFile" yaml:"publicKeyFile"`
	Subject       string `json:"subject" yaml:"subject"`
	Principal     string `json:"principal" yaml:"principal"`
}

// GoogleTokenBrokerConfig constrains all browser token requests on this runner.
type GoogleTokenBrokerConfig struct {
	Enabled                bool     `json:"enabled" yaml:"enabled"`
	Source                 string   `json:"source" yaml:"source"`
	TokenMode              string   `json:"tokenMode" yaml:"tokenMode"`
	ServiceAccountEmail    string   `json:"serviceAccountEmail" yaml:"serviceAccountEmail"`
	AllowedScopes          []string `json:"allowedScopes" yaml:"allowedScopes"`
	DefaultLifetimeSeconds int      `json:"defaultLifetimeSeconds" yaml:"defaultLifetimeSeconds"`
	MaxLifetimeSeconds     int      `json:"maxLifetimeSeconds" yaml:"maxLifetimeSeconds"`
}

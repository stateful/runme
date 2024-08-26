package config

var defaults Config

func init() {
	// todo(sebastian): use inline file for easier editing
	yaml := []byte(`version: v1alpha1

# Settings that apply on at the project level.
project:
  # Indicate the root of the runme project. "." means that
  # the project root directory will be used.
  root: "."
  # If true, the project root will be searched upwards starting from "dir".
  # If found, the repo root will be used as the project root.
  find_repo_upward: true
  ignore:
    - "node_modules"
    - ".venv"
  disable_gitignore: false

  # It's possible to point the project at a single file.
  # filename: "README.md"

  # List of dotenv files to load.
  env:
    use_system_env: false
    sources:
      - ".env"
      - ".env.local"

  # The list of filters to apply to blocks.
  # "condition" must return a boolean value.
  # You can learn about the syntax at https://expr-lang.org/docs/language-definition.
  # Available fields are defined in [config.FilterDocumentEnv] and [config.FilterBlockEnv].
  # filters:
  #   # Do not allow unnamed code blocks.
  #   # - type: "FILTER_TYPE_BLOCK"
  #   #   condition: "is_named"
  #   # Do not allow code blocks without a language.
  #   - type: "FILTER_TYPE_BLOCK"
  #     condition: "language != ''"
  #   # Do not allow code blocks starting with "test".
  #   - type: "FILTER_TYPE_BLOCK"
  #     condition: "!hasPrefix(name, 'test')"

runtime:
  # Optional Docker configuration to run code blocks in a container.
  docker:
    enabled: false
    image: runme-runtime:latest
    build:
      context: ./experimental/docker
      dockerfile: Dockerfile

server:
  # Also unix:///path/to/file.sock is supported.
  address: localhost:7998
  tls:
    enabled: true
    # If not specified, default paths will be used.
    # cert_file: "/path/to/cert.pem"
    # key_file: "/path/to/key.pem"

log:
  enabled: false
  path: "/tmp/runme.log"
  verbose: false
`)

	cfg, err := ParseYAML(yaml)
	if err != nil {
		panic(err)
	}

	defaults = *cfg
}

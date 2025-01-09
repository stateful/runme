module github.com/stateful/runme/v3

go 1.23

toolchain go1.23.2

// replace github.com/stateful/godotenv => ../godotenv

require (
	cloud.google.com/go/secretmanager v1.14.3
	github.com/Masterminds/semver/v3 v3.3.1
	github.com/Microsoft/go-winio v0.6.2
	github.com/atotto/clipboard v0.1.4
	github.com/charmbracelet/bubbletea v1.2.4
	github.com/charmbracelet/lipgloss v1.0.0
	github.com/cli/cli/v2 v2.65.0
	github.com/cli/go-gh/v2 v2.11.2
	github.com/containerd/console v1.0.4
	github.com/creack/pty v1.1.24
	github.com/docker/docker v27.4.1+incompatible
	github.com/expr-lang/expr v1.16.9
	github.com/fatih/color v1.18.0
	github.com/fullstorydev/grpcurl v1.9.2
	github.com/gabriel-vasile/mimetype v1.4.8
	github.com/go-git/go-billy/v5 v5.6.1
	github.com/gobwas/glob v0.2.3
	github.com/golang/mock v1.6.0
	github.com/google/go-github/v45 v45.2.0
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/graphql-go/graphql v0.8.1
	github.com/jhump/protoreflect v1.17.0
	github.com/mattn/go-isatty v0.0.20
	github.com/mgutz/ansi v0.0.0-20200706080929-d51e80ef957d
	github.com/muesli/cancelreader v0.2.2
	github.com/oklog/ulid/v2 v2.1.0
	github.com/opencontainers/image-spec v1.1.0
	github.com/otiai10/copy v1.14.1
	github.com/rogpeppe/go-internal v1.13.1
	github.com/rwtodd/Go.Sed v0.0.0-20240405174034-bb8ed5da0fd0
	github.com/stateful/godotenv v0.0.0-20240309032207-c7bc0b812915
	github.com/vektah/gqlparser/v2 v2.5.21
	github.com/xo/dburl v0.23.2
	github.com/yuin/goldmark v1.7.8
	go.uber.org/dig v1.18.0
	go.uber.org/multierr v1.11.0
	golang.org/x/exp v0.0.0-20250106191152-7588d65b2ba8
	golang.org/x/oauth2 v0.25.0
	golang.org/x/sys v0.29.0
	golang.org/x/term v0.28.0
	google.golang.org/api v0.216.0
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250106144421-5f5ef82da422
	google.golang.org/protobuf v1.36.2
	mvdan.cc/sh/v3 v3.10.0
)

require (
	cloud.google.com/go/auth v0.14.0 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.7 // indirect
	cloud.google.com/go/compute/metadata v0.6.0 // indirect
	cloud.google.com/go/iam v1.3.1 // indirect
	dario.cat/mergo v1.0.1 // indirect
	github.com/aymanbagabas/go-osc52/v2 v2.0.1 // indirect
	github.com/bufbuild/protocompile v0.14.1 // indirect
	github.com/charmbracelet/x/ansi v0.6.0 // indirect
	github.com/charmbracelet/x/term v0.2.1 // indirect
	github.com/cncf/xds/go v0.0.0-20241223141626-cff3c89139a3 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/cyphar/filepath-securejoin v0.3.6 // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/go-connections v0.5.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/envoyproxy/go-control-plane/envoy v1.32.3 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.1.0 // indirect
	github.com/erikgeiser/coninput v0.0.0-20211004153227-1c3628e74d0f // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.4 // indirect
	github.com/googleapis/gax-go/v2 v2.14.1 // indirect
	github.com/mmcloughlin/avo v0.6.0 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/term v0.5.0 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/otiai10/mint v1.6.3 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.58.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.58.0 // indirect
	go.opentelemetry.io/otel v1.33.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.24.0 // indirect
	go.opentelemetry.io/otel/metric v1.33.0 // indirect
	go.opentelemetry.io/otel/trace v1.33.0 // indirect
	golang.org/x/mod v0.22.0 // indirect
	golang.org/x/net v0.34.0 // indirect
	golang.org/x/time v0.9.0 // indirect
	google.golang.org/genproto v0.0.0-20250106144421-5f5ef82da422 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250106144421-5f5ef82da422 // indirect
	gotest.tools/v3 v3.5.1 // indirect
)

require (
	github.com/ProtonMail/go-crypto v1.1.4 // indirect
	github.com/briandowns/spinner v1.23.1 // indirect
	github.com/cli/go-gh v1.2.1
	github.com/cli/safeexec v1.0.1 // indirect
	github.com/cloudflare/circl v1.5.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/go-git/gcfg v1.5.1-0.20230307220236-3a3c6141e376 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/kevinburke/ssh_config v1.2.0 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-localereader v0.0.1 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/muesli/ansi v0.0.0-20230316100256-276c6243b2f6 // indirect
	github.com/muesli/reflow v0.3.0 // indirect
	github.com/muesli/termenv v0.15.2 // indirect
	github.com/pjbgf/sha1cd v0.3.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/sahilm/fuzzy v0.1.1 // indirect
	github.com/sergi/go-diff v1.3.2-0.20230802210424-5b0b94c5c0d3 // indirect
	github.com/skeema/knownhosts v1.3.0 // indirect
	github.com/xanzy/ssh-agent v0.3.3 // indirect
	golang.org/x/crypto v0.32.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	golang.org/x/tools v0.29.0 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/Khan/genqlient v0.7.0
	github.com/charmbracelet/bubbles v0.20.0
	github.com/elliotchance/orderedmap v1.8.0
	github.com/go-git/go-git/v5 v5.13.1
	github.com/go-playground/assert/v2 v2.2.0
	github.com/go-playground/validator/v10 v10.23.0
	github.com/golang-jwt/jwt/v4 v4.5.1
	github.com/google/go-cmp v0.6.0
	github.com/google/uuid v1.6.0
	github.com/hashicorp/golang-lru/v2 v2.0.7
	github.com/henvic/httpretty v0.1.4
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/pelletier/go-toml/v2 v2.2.3
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c
	github.com/pkg/errors v0.9.1
	github.com/pkg/term v1.2.0-beta.2.0.20211217091447-1a4a3b719465
	github.com/spf13/cobra v1.8.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.10.0
	go.uber.org/zap v1.27.0
	golang.org/x/sync v0.10.0
	google.golang.org/grpc v1.69.2
)

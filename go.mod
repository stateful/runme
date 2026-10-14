module github.com/runmedev/runme/v3

// This is the v3 module. Always use the /v3 suffix when installing:
//   go install github.com/runmedev/runme/v3@latest
// This ensures you get the latest v3.x.x version, not the old v1.x.x versions.

go 1.27.0

require (
	buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go v1.36.12-20260825204119-511051f7f437.2
	connectrpc.com/connect v1.21.0
	connectrpc.com/cors v0.1.0
	connectrpc.com/grpchealth v1.5.0
	connectrpc.com/otelconnect v0.10.0
	github.com/Khan/genqlient v0.8.1
	github.com/Masterminds/semver/v3 v3.5.0
	github.com/atotto/clipboard v0.1.4
	github.com/charmbracelet/bubbles v1.0.0
	github.com/charmbracelet/bubbletea v1.3.10
	github.com/charmbracelet/lipgloss v1.1.1-0.20250404203927-76690c660834
	github.com/cli/cli/v2 v2.101.0
	github.com/cli/go-gh v1.2.1
	github.com/cli/go-gh/v2 v2.16.1
	github.com/containerd/console v1.0.5
	github.com/creack/pty v1.1.24
	github.com/docker/docker v28.5.2+incompatible
	github.com/elliotchance/orderedmap v1.8.0
	github.com/expr-lang/expr v1.17.8
	github.com/fatih/color v1.19.0
	github.com/fullstorydev/grpcurl v1.9.4
	github.com/gabriel-vasile/mimetype v1.4.15
	github.com/go-git/go-billy/v5 v5.9.1
	github.com/go-git/go-git/v5 v5.19.2
	github.com/go-logr/logr v1.4.4
	github.com/go-logr/zapr v1.3.0
	github.com/go-playground/assert/v2 v2.2.0
	github.com/gobwas/glob v1.0.0
	github.com/golang-jwt/jwt/v4 v4.5.2
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/golang/mock v1.7.0-rc.1
	github.com/google/go-cmp v0.7.0
	github.com/google/go-github/v45 v45.2.0
	github.com/google/uuid v1.6.0
	github.com/gorilla/websocket v1.5.3
	github.com/henvic/httpretty v0.2.0
	github.com/jhump/protoreflect v1.18.1
	github.com/jlewi/monogo v0.0.0-20241216141120-2e83e825aa81
	github.com/mattn/go-isatty v0.0.24
	github.com/mgutz/ansi v0.0.0-20200706080929-d51e80ef957d
	github.com/muesli/cancelreader v0.2.2
	github.com/oklog/ulid/v2 v2.1.2
	github.com/opencontainers/image-spec v1.1.1
	github.com/otiai10/copy v1.14.1
	github.com/pelletier/go-toml/v2 v2.4.3
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c
	github.com/pkg/errors v0.9.1
	github.com/pkg/term v1.2.0-beta.2.0.20211217091447-1a4a3b719465
	github.com/prometheus/client_golang v1.24.1
	github.com/redpanda-data/protoc-gen-go-mcp v0.0.0-20260729122341-6334690a62b6
	github.com/rogpeppe/go-internal v1.16.0
	github.com/rs/cors v1.11.1
	github.com/runmedev/owl v1.0.1
	github.com/spf13/cobra v1.10.2
	github.com/spf13/pflag v1.0.10
	github.com/spf13/viper v1.21.0
	github.com/stateful/godotenv v0.0.0-20240309032207-c7bc0b812915
	github.com/stretchr/testify v1.12.1
	github.com/vektah/gqlparser/v2 v2.5.62
	github.com/yuin/goldmark v1.8.6
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.71.0
	go.opentelemetry.io/otel v1.46.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.46.0
	go.opentelemetry.io/otel/sdk v1.46.0
	go.opentelemetry.io/otel/trace v1.46.0
	go.uber.org/dig v1.19.0
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.28.0
	golang.org/x/net v0.59.0
	golang.org/x/oauth2 v0.37.0
	golang.org/x/sync v0.23.0
	golang.org/x/sys v0.48.0
	golang.org/x/term v0.46.0
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260928230214-8a89bd6388cc
	google.golang.org/grpc v1.84.0
	google.golang.org/protobuf v1.36.12
	gopkg.in/yaml.v3 v3.0.1
	mvdan.cc/sh/v3 v3.14.1
	sigs.k8s.io/kustomize/kyaml v0.21.1
)

require (
	buf.build/gen/go/redpandadata/common/protocolbuffers/go v1.36.12-20260323171043-6e06f84ad823.2 // indirect
	cloud.google.com/go v0.123.0 // indirect
	cloud.google.com/go/auth v0.24.0 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.3.0 // indirect
	cloud.google.com/go/compute/metadata v0.10.0 // indirect
	cloud.google.com/go/iam v1.14.0 // indirect
	cloud.google.com/go/logging v1.20.0 // indirect
	cloud.google.com/go/longrunning v1.3.0 // indirect
	cloud.google.com/go/secretmanager v1.22.0 // indirect
	codeberg.org/chavacava/garif v0.2.1 // indirect
	dario.cat/mergo v1.0.2 // indirect
	github.com/BurntSushi/toml v1.6.0 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/ProtonMail/go-crypto v1.5.2 // indirect
	github.com/anthropics/anthropic-sdk-go v1.66.0 // indirect
	github.com/atombender/go-jsonschema v0.24.1 // indirect
	github.com/aymanbagabas/go-osc52/v2 v2.0.1 // indirect
	github.com/bahlo/generic-list-go v0.2.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/briandowns/spinner v1.23.2 // indirect
	github.com/buger/jsonparser v1.6.1 // indirect
	github.com/ccojocar/zxcvbn-go v1.0.4 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/charmbracelet/colorprofile v0.4.3 // indirect
	github.com/charmbracelet/x/ansi v0.11.8 // indirect
	github.com/charmbracelet/x/cellbuf v0.0.15 // indirect
	github.com/charmbracelet/x/term v0.2.2 // indirect
	github.com/cli/safeexec v1.0.1 // indirect
	github.com/clipperhouse/displaywidth v0.11.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/cloudflare/circl v1.6.5 // indirect
	github.com/cncf/xds/go v0.0.0-20260202195803-dba9d589def2 // indirect
	github.com/containerd/errdefs v1.0.0 // indirect
	github.com/containerd/errdefs/pkg v0.3.0 // indirect
	github.com/cyphar/filepath-securejoin v0.7.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/go-connections v0.8.1 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/envoyproxy/go-control-plane/envoy v1.39.0 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.3.3 // indirect
	github.com/erikgeiser/coninput v0.0.0-20211004153227-1c3628e74d0f // indirect
	github.com/fatih/structtag v1.2.0 // indirect
	github.com/felixge/httpsnoop v1.1.0 // indirect
	github.com/fsnotify/fsnotify v1.10.1 // indirect
	github.com/go-errors/errors v1.5.1 // indirect
	github.com/go-git/gcfg v1.5.1-0.20230307220236-3a3c6141e376 // indirect
	github.com/go-jose/go-jose/v4 v4.1.5 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/jsonpointer v1.0.2 // indirect
	github.com/go-openapi/jsonreference v1.0.3 // indirect
	github.com/go-openapi/swag v0.29.2 // indirect
	github.com/go-openapi/swag/cmdutils v0.29.2 // indirect
	github.com/go-openapi/swag/conv v0.29.2 // indirect
	github.com/go-openapi/swag/fileutils v0.29.2 // indirect
	github.com/go-openapi/swag/jsonutils v0.29.2 // indirect
	github.com/go-openapi/swag/loading v0.29.2 // indirect
	github.com/go-openapi/swag/mangling v0.29.2 // indirect
	github.com/go-openapi/swag/netutils v0.29.2 // indirect
	github.com/go-openapi/swag/pools v0.29.2 // indirect
	github.com/go-openapi/swag/stringutils v0.29.2 // indirect
	github.com/go-openapi/swag/typeutils v0.29.2 // indirect
	github.com/go-openapi/swag/yamlutils v0.29.2 // indirect
	github.com/go-playground/locales v0.14.2 // indirect
	github.com/go-playground/universal-translator v0.18.2 // indirect
	github.com/go-playground/validator/v10 v10.30.5 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/goccy/go-yaml v1.19.2 // indirect
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/gnostic-models v0.7.1 // indirect
	github.com/google/go-querystring v1.2.0 // indirect
	github.com/google/s2a-go v0.1.10 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.22 // indirect
	github.com/googleapis/gax-go/v2 v2.26.2 // indirect
	github.com/gookit/color v1.6.1 // indirect
	github.com/graphql-go/graphql v0.8.1 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.31.0 // indirect
	github.com/hashicorp/go-version v1.9.0 // indirect
	github.com/icholy/gomajor v0.15.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/invopop/jsonschema v0.14.0 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/jhump/protoreflect/v2 v2.0.0-beta.2 // indirect
	github.com/kevinburke/ssh_config v1.6.0 // indirect
	github.com/klauspost/cpuid/v2 v2.4.0 // indirect
	github.com/leodido/go-urn v1.5.0 // indirect
	github.com/lucasb-eyer/go-colorful v1.4.1 // indirect
	github.com/mattn/go-colorable v0.1.15 // indirect
	github.com/mattn/go-localereader v0.0.1 // indirect
	github.com/mattn/go-runewidth v0.0.30 // indirect
	github.com/mgechev/dots v1.0.0 // indirect
	github.com/mgechev/revive v1.16.0 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/sys/atomicwriter v0.1.0 // indirect
	github.com/moby/term v0.5.0 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/muesli/ansi v0.0.0-20230316100256-276c6243b2f6 // indirect
	github.com/muesli/reflow v0.3.0 // indirect
	github.com/muesli/termenv v0.16.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/openai/openai-go/v3 v3.52.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/otiai10/mint v1.6.3 // indirect
	github.com/pb33f/ordered-map/v2 v2.3.1 // indirect
	github.com/petermattis/goid v0.0.0-20260918085751-abfca077860b // indirect
	github.com/pjbgf/sha1cd v0.6.0 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/prometheus/client_model v0.6.3 // indirect
	github.com/prometheus/common v0.72.0 // indirect
	github.com/prometheus/procfs v0.22.0 // indirect
	github.com/redpanda-data/common-go/api v0.0.0-20260922181238-9566b7beb637 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/sagikazarmark/locafero v0.12.0 // indirect
	github.com/sahilm/fuzzy v0.1.3 // indirect
	github.com/sanity-io/litter v1.5.8 // indirect
	github.com/securego/gosec/v2 v2.29.0 // indirect
	github.com/sergi/go-diff v1.4.0 // indirect
	github.com/skeema/knownhosts v1.3.3 // indirect
	github.com/sosodev/duration v1.4.0 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/spiffe/go-spiffe/v2 v2.8.2 // indirect
	github.com/standard-webhooks/standard-webhooks/libraries v0.0.1 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/tidwall/gjson v1.19.0 // indirect
	github.com/tidwall/match v1.2.0 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/sjson v1.2.5 // indirect
	github.com/xanzy/ssh-agent v0.3.3 // indirect
	github.com/xo/dburl v0.38.0 // indirect
	github.com/xo/terminfo v1.2.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.71.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.46.0 // indirect
	go.opentelemetry.io/otel/metric v1.46.0 // indirect
	go.opentelemetry.io/proto/otlp v1.11.0 // indirect
	go.yaml.in/yaml/v2 v2.4.4 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	go.yaml.in/yaml/v4 v4.0.0-rc.6 // indirect
	golang.org/x/crypto v0.57.0 // indirect
	golang.org/x/exp/typeparams v0.0.0-20231108232855-2478ac86f678 // indirect
	golang.org/x/mod v0.41.0 // indirect
	golang.org/x/telemetry v0.0.0-20260908163034-4bcc4b2ee518 // indirect
	golang.org/x/text v0.42.0 // indirect
	golang.org/x/time v0.16.0 // indirect
	golang.org/x/tools v0.50.0 // indirect
	google.golang.org/api v0.299.0 // indirect
	google.golang.org/genai v1.69.0 // indirect
	google.golang.org/genproto v0.0.0-20260928230214-8a89bd6388cc // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260928230214-8a89bd6388cc // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gvisor.dev/gvisor v0.0.0-20260901055435-6ed83306f111 // indirect
	honnef.co/go/tools v0.8.1 // indirect
	k8s.io/kube-openapi v0.0.0-20260928204700-0115328ef16b // indirect
	mvdan.cc/gofumpt v0.11.0 // indirect
)

tool (
	github.com/atombender/go-jsonschema
	github.com/icholy/gomajor
	github.com/mgechev/revive
	github.com/securego/gosec/v2/cmd/gosec
	golang.org/x/tools/cmd/goimports
	gvisor.dev/gvisor/tools/checklocks/cmd/checklocks
	honnef.co/go/tools/cmd/staticcheck
	mvdan.cc/gofumpt
)

SHELL := /bin/bash

GO_ROOT := $(shell go env GOROOT)
GIT_SHA := $(shell git rev-parse HEAD)
GIT_SHA_SHORT := $(shell git rev-parse --short HEAD)
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
VERSION := $(shell git describe --tags)-$(GIT_SHA_SHORT)
LDFLAGS := -s -w \
	-X 'github.com/stateful/runme/v3/internal/version.BuildDate=$(DATE)' \
	-X 'github.com/stateful/runme/v3/internal/version.BuildVersion=$(subst v,,$(VERSION))' \
	-X 'github.com/stateful/runme/v3/internal/version.Commit=$(GIT_SHA)'

LDTESTFLAGS := -X 'github.com/stateful/runme/v3/internal/version.BuildVersion=$(subst v,,$(VERSION))'

ifeq ($(RUNME_EXT_BASE),)
RUNME_EXT_BASE := "../vscode-runme"
endif

.PHONY: build
build: BUILD_OUTPUT ?= runme
build:
	CGO_ENABLED=0 go build -o $(BUILD_OUTPUT) -ldflags="$(LDFLAGS)" main.go

.PHONY: wasm
wasm: WASM_OUTPUT ?= examples/web
wasm:
	cp $(GO_ROOT)/misc/wasm/wasm_exec.js $(WASM_OUTPUT)
	GOOS=js GOARCH=wasm go build -o $(WASM_OUTPUT)/runme.wasm -ldflags="$(LDFLAGS)" ./web

.PHONY: test/execute
test/execute: PKGS ?= "./..."
test/execute: RUN ?= .*
test/execute: RACE ?= false
test/execute: TAGS ?= "" # e.g. TAGS="test_with_docker"
# It depends on the build target because the runme binary
# is used for tests, for example, "runme env dump".
test/execute: build
	TZ=UTC go test -ldflags="$(LDTESTFLAGS)" -run="$(RUN)" -tags="$(TAGS)" -timeout=60s -race=$(RACE) $(PKGS)

.PHONY: test/coverage
test/coverage: PKGS ?= "./..."
test/coverage: RUN ?= .*
test/coverage: TAGS ?= "" # e.g. TAGS="test_with_docker"
# It depends on the build target because the runme binary
# is used for tests, for example, "runme env dump".
test/coverage: build
	TZ=UTC go test -ldflags="$(LDTESTFLAGS)" -run="$(RUN)" -tags="$(TAGS)" -timeout=90s -covermode=atomic -coverprofile=cover.out -coverpkg=./... $(PKGS)

.PHONY: test
test: test/execute

.PHONY: test-coverage
test-coverage: test/coverage

.PHONY: test-docker
test-docker: test-docker/setup test-docker/run

.PHONY: test-docker/setup
test-docker/setup:
	docker build \
		-t runme-test-env:latest \
		-f ./docker/runme-test-env.Dockerfile .
	docker volume create dev.runme.test-env-gocache

.PHONY: test-docker/cleanup
test-docker/cleanup:
	docker volume rm dev.runme.test-env-gocache

.PHONY: test-docker/run
test-docker/run:
	docker run --rm \
		-v $(shell pwd):/workspace \
		-v dev.runme.test-env-gocache:/root/.cache/go-build \
		runme-test-env:latest

.PHONY: test/update-snapshots
test/update-snapshots:
	@TZ=UTC UPDATE_SNAPSHOTS=true go test ./...

.PHONY: test/robustness
test/robustness:
	./runme --version
	find "$$GOPATH/pkg/mod/github.com" -name "*.md" | grep -v "\/\." | xargs dirname | uniq | xargs -n1 -I {} ./runme fmt --project {} > /dev/null

.PHONY: coverage/html
test/coverage/html:
	go tool cover -html=cover.out

.PHONY: coverage/func
test/coverage/func:
	go tool cover -func=cover.out

.PHONY: fmt
fmt:
	@gofumpt -w .

.PHONY: lint
lint:
	@gofumpt -d .
	@revive \
		-config revive.toml \
		-formatter stylish \
		-exclude integration/subject/... \
		./...
	@staticcheck ./...
	@gosec -quiet -exclude=G110,G204,G304,G404 -exclude-generated ./...

.PHONY: pre-commit
pre-commit: build wasm test lint
	pre-commit run --all-files

.PHONY: install/dev
install/dev:
	go install github.com/mgechev/revive@v1.3.9
	go install github.com/securego/gosec/v2/cmd/gosec@v2.20.0
	go install honnef.co/go/tools/cmd/staticcheck@v0.5.1
	go install mvdan.cc/gofumpt@v0.7.0
	go install github.com/icholy/gomajor@v0.13.1
	go install github.com/stateful/go-proto-gql/protoc-gen-gql@latest
	go install github.com/atombender/go-jsonschema@v0.16.0

.PHONY: install/goreleaser
install/goreleaser:
	go install github.com/goreleaser/goreleaser@v1.26.2

.PHONY: proto/generate
proto/generate:
	buf lint
	buf format -w
	buf generate

.PHONY: proto/clean
proto/clean:
	rm -rf pkg/api/gen/proto

.PHONY: proto/dev
proto/dev: build proto/clean proto/generate
	rm -rf $(RUNME_EXT_BASE)/node_modules/@buf/stateful_runme.community_timostamm-protobuf-ts/runme
	cp -vrf pkg/api/gen/proto/ts/runme $(RUNME_EXT_BASE)/node_modules/@buf/stateful_runme.community_timostamm-protobuf-ts

.PHONY: proto/dev/reset
proto/dev/reset:
	rm -rf $(RUNME_EXT_BASE)/node_modules/@buf/stateful_runme.community_timostamm-protobuf-ts
	cd $(RUNME_EXT_BASE) && runme run setup

# Remember to set up buf registry beforehand.
# More: https://docs.buf.build/bsr/authentication
.PHONY: proto/publish
proto/publish:
	@cd ./pkg/api/proto && buf push

.PHONY: schema/generate
schema/generate:
	go-jsonschema -t \
		-p config \
		--tags "json,yaml" \
		-o internal/config/config_schema.go \
		internal/config/config.schema.json

.PHONY: release
release: install/goreleaser
	@goreleaser check
	@goreleaser release --snapshot --clean

.PHONY: release/publish
release/publish: install/goreleaser
	@goreleaser release

.PHONY: update-gql-schema
update-gql-schema:
	@go run ./cmd/gqltool/main.go > ./internal/client/graphql/schema/introspection_query_result.json
	@npm install --prefix internal/client/graphql/schema
	@cd ./internal/client/graphql/schema && npm run convert

.PHONY: generate
generate:
	go generate ./...

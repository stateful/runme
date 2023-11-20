SHELL := /bin/bash

GO_ROOT := $(shell go env GOROOT)
GIT_SHA := $(shell git rev-parse HEAD)
GIT_SHA_SHORT := $(shell git rev-parse --short HEAD)
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
VERSION := $(shell git describe --tags)-$(GIT_SHA_SHORT)
LDFLAGS := -s -w \
	-X 'github.com/stateful/runme/internal/version.BuildDate=$(DATE)' \
	-X 'github.com/stateful/runme/internal/version.BuildVersion=$(subst v,,$(VERSION))' \
	-X 'github.com/stateful/runme/internal/version.Commit=$(GIT_SHA)'

ifeq ($(RUNME_EXT_BASE),)
RUNME_EXT_BASE := "../vscode-runme"
endif

.PHONY: build
build:
	go build -o runme -ldflags="$(LDFLAGS)" main.go

.PHONY: wasm
wasm: WASM_OUTPUT ?= examples/web
wasm:
	cp $(GO_ROOT)/misc/wasm/wasm_exec.js $(WASM_OUTPUT)
	GOOS=js GOARCH=wasm go build -o $(WASM_OUTPUT)/runme.wasm -ldflags="$(LDFLAGS)" ./web

.PHONY: test
test: build
	@TZ=UTC go test -timeout=30s ./...

.PHONY: test/update-snapshots
test/update-snapshots:
	@TZ=UTC UPDATE_SNAPSHOTS=true go test ./...

.PHONY: test/robustness
test/robustness:
	@cd integration/subject && npm install --include=dev
	find . -name "README.md" | grep -v "\/\." | xargs dirname | uniq | xargs -n1 -I {} ./runme fmt --chdir {} > /dev/null

.PHONY: fmt
fmt:
	@gofumpt -w .

.PHONY: lint
lint:
	@revive -config revive.toml -formatter stylish ./...

.PHONY: pre-commit
pre-commit: build test lint
	pre-commit run --all-files

.PHONY: install/dev
install/dev:
	go install github.com/mgechev/revive@v1.2.3
	go install github.com/securego/gosec/v2/cmd/gosec@v2.17.0
	go install honnef.co/go/tools/cmd/staticcheck@v0.4.3
	go install mvdan.cc/gofumpt@v0.3.1
	go install github.com/icholy/gomajor@v0.9.5

.PHONY: install/goreleaser
install/goreleaser:
	go install github.com/goreleaser/goreleaser@v1.15.2

.PHONY: proto/generate
proto/generate:
	buf lint
	buf format -w
	buf generate

.PHONY: proto/clean
proto/clean:
	rm -rf internal/gen/proto

.PHONY: proto/dev
proto/dev: build proto/clean proto/generate
	cp -vrf internal/gen/proto/ts/runme $(RUNME_EXT_BASE)/node_modules/@buf/stateful_runme.community_timostamm-protobuf-ts
	find $(RUNME_EXT_BASE)/node_modules/@buf/stateful_runme.community_timostamm-protobuf-ts -name "*.ts" | grep -v ".d.ts" | xargs rm -f

.PHONY: proto/dev/reset
proto/dev/reset:
	rm -rf $(RUNME_EXT_BASE)/node_modules/@buf/stateful_runme.community_timostamm-protobuf-ts
	cd $(RUNME_EXT_BASE) && runme run setup

# Remember to set up buf registry beforehand.
# More: https://docs.buf.build/bsr/authentication
.PHONY: proto/publish
proto/publish:
	@cd ./internal/api && buf push

.PHONY: release
release: install/goreleaser
	@goreleaser check
	@goreleaser release --snapshot --clean

.PHONY: release/publish
release/publish: install/goreleaser
	@goreleaser release

.PHONY: update-gql-schema
update-gql-schema:
	@go run ./cmd/gqltool/main.go > ./client/graphql/schema/introspection_query_result.json
	@cd ./client/graphql/schema && npm run convert

.PHONY: generate
generate:
	go generate ./...

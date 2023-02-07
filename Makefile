SHELL := /bin/bash

GO_ROOT := $(shell go env GOROOT)
GIT_SHA := $(shell git rev-parse HEAD)
GIT_SHA_SHORT := $(shell git rev-parse --short HEAD)
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
VERSION := $(shell git describe --tags)-$(GIT_SHA_SHORT)
LDFLAGS := -s -w \
	-X 'main.BuildDate=$(DATE)' \
	-X 'main.BuildVersion=$(subst v,,$(VERSION))' \
	-X 'main.Commit=$(GIT_SHA)'

.PHONY: build
build:
	go build -o runme -ldflags="$(LDFLAGS)" main.go

.PHONY: wasm
wasm: WASM_OUTPUT ?= examples/web
wasm:
	cp $(GO_ROOT)/misc/wasm/wasm_exec.js $(WASM_OUTPUT)
	GOOS=js GOARCH=wasm go build -o $(WASM_OUTPUT)/runme.wasm -ldflags="$(LDFLAGS)" ./web

.PHONY: test
test:
	@TZ=UTC go test ./...

.PHONY: test/update-snapshots
test/update-snapshots:
	@TZ=UTC UPDATE_SNAPSHOTS=true go test ./...

.PHONY: test/robustness
test/robustness:
	@cd integration/subject && npm install --include=dev
	find . -name "README.md" | grep -v "\/\." | xargs dirname | uniq | xargs -n1 -I {} ./runme fmt --chdir {} - > /dev/null

.PHONY: fmt
fmt:
	@gofumpt -w .

.PHONY: lint
lint:
	@revive -config revive.toml -formatter stylish ./...

.PHONY: install/dev
install/dev:
	go install github.com/mgechev/revive@v1.2.3
	go install github.com/securego/gosec/v2/cmd/gosec@v2.12.0
	go install honnef.co/go/tools/cmd/staticcheck@v0.3.3
	go install mvdan.cc/gofumpt@v0.3.1

.PHONY: install/goreleaser
install/goreleaser:
	go install github.com/goreleaser/goreleaser@v1.10.2

.PHONY: proto/generate
proto/generate:
	buf generate

.PHONY: proto/clean
proto/clean:
	rm -rf internal/gen/proto

# Remember to set up buf registry beforehand.
# More: https://docs.buf.build/bsr/authentication
.PHONY: proto/publish
proto/publish:
	@cd ./internal/api && buf push

.PHONY: release
release: install/goreleaser
	@goreleaser check
	@goreleaser release --snapshot --rm-dist

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
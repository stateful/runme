---
runme:
  id: 01HF7BT3HF9WY615MNGVPCSFMV
  version: v3
---

# Contributing to `runme`

**Thank you for your interest in `runme`. Your contributions are highly welcome.**

There are multiple ways of getting involved:

- [Report a bug](#report-a-bug)
- [Suggest a feature](#suggest-a-feature)
- [Contribute code](#contribute-code)

## Report a bug

Reporting bugs is one of the best ways to contribute. Before creating a bug report, please check that an [issue](/issues) reporting the same problem does not already exist. If there is such an issue, you may add your information as a comment.

To report a new bug you should open an issue that summarizes the bug and set the label to "bug".

If you want to provide a fix along with your bug report: That is great! In this case please send us a pull request as described in section [Contribute Code](#contribute-code).

## Suggest a feature

To request a new feature you should open an [issue](../../issues/new) and summarize the desired functionality and its use case. Set the issue label to "feature".

## Contribute code

This is an outline of what the workflow for code contributions looks like

- Check the list of open [issues](../../issues). Either assign an existing issue to yourself, or
   create a new one that you would like work on and discuss your ideas and use cases.

It is always best to discuss your plans beforehand, to ensure that your contribution is in line with our goals.

- Fork the repository on GitHub
- Create a topic branch from where you want to base your work. This is usually `main`
- Open a new draft pull request and outline what you will be contributing
- Make commits of logical units
- Write [good commit messages](https://cbea.ms/git-commit/)
- Push your changes to a topic branch in your fork of the repository
- As you push your changes, update the pull request with new information and tasks as you complete them
- Project maintainers might comment on your work as you progress
- When you are done, remove the PR's draft status and ping the maintainers for a review

## Prerequisites

This project uses a `Makefile` to manage build scripts. You will need `make` installed to run these scripts. See [Makefile](/Makefile) for a list of possible commands and what they do.

You will need to have a `go` installation - ideally compatible with the project's current go version (see [go.mod](/go.mod)).

### macOS

If you are using [`homebrew`](https://brew.sh/), you can install the required system modules with the following command:

```sh {"id":"01HF7BT3HEQBTBM9SSTB6V9WKN","interactive":"false"}
brew bundle --no-lock
```

In order to use `make`, install [apple developer tools](https://developer.apple.com/xcode/resources/).

## Setup

To install required CLI tools for development:

```sh {"id":"01HF7BT3HEQBTBM9SSTBPD50X3","interactive":"false"}
make install/dev
```

Make sure to export the global path for Go packages into your environment. For Mac and Linux, just run:

```sh {"id":"01HF7BT3HEQBTBM9SSTCFZ708X"}
export GOPATH="$HOME/go"
PATH="$GOPATH/bin:$PATH"
```

## Build

This codebase is used for a few different purposes, and so there's quite a lot of architecture involved.

Following is some documentation on the different build targets `runme` has, and what is required to get them to build/run.

### Build Targets

#### CLI

Build the CLI:

```sh {"id":"01HF7BT3HEQBTBM9SSTFD1C6N4","interactive":"false","name":"build"}
make build
```

This builds the CLI binary for the current platform to an executable file "runme" in the root directory. (You can change the output with the `-o` flag). After this command you can access the compiled binary from the root directory in your workspace, e.g.:

```sh {"id":"01HF7BT3HEQBTBM9SSTG26T7EJ"}
./runme --version
# outputs: "runme version 1.3.0-27-g3cca8a6-3cca8a6 (3cca8a6e7d34f401c1bdd99828a7fac5b1d8fac9) on 2023-07-31T16:49:57Z"
```

#### WASM

WASM is built with:

```sh {"id":"01HF7BT3HEQBTBM9SSTH3HC2DS"}
make wasm
ls -la examples/web/runme.wasm
```

This builds the wasm file to `examples/web/runme.wasm`.

## Install Dev Tools

To install tools like `gofumpt` and `revive` which are used for development (e.g. linting) run

```sh {"id":"01J5P9MKFZ4SRS1VE6J62HFKP0","name":"setup"}
make install/dev
```

You will need the [pre-commit](https://pre-commit.com/) tool to run the pre-commit hooks. You can install it with:

```sh {"id":"01J5P9MKFZ4SRS1VE6J8XKD8ZM"}
python3 -m pip install pre-commit
```

## Linting

Like many complex go projects, this project uses a variety of linting tools to ensure code quality and prevent regressions! The main linter (revive) can be run with:

```sh {"id":"01HF7BT3HEQBTBM9SSTKQENPT3","interactive":"false"}
make lint
```

The rest of the project's linting suite can be run with:

```sh {"id":"01HF7BT3HEQBTBM9SSTPGWHF1K"}
pre-commit run --files */**
```

## Testing

Tests are run with Go's default test runner wrapped in Makefile targets. So, for example, you can run all tests with:

```sh {"id":"01HF7BT3HEQBTBM9SSTS88ZSCF","name":"test","terminalRows":"15"}
go clean -testcache
TAGS="test_with_docker" make test
```

Please notice that our tests include integration tests which depend on additional software like Python or node.js. If you don't want to install them or tests fail because of different versions, you can run all tests in a Docker container:

```sh {"id":"01J5P9MKFZ4SRS1VE6JBKG3EAK","name":"test-docker","terminalRows":"15"}
make test-docker
```

## Development

To run the server in dev mode with predictable port and TLS credentials location:

```sh {"background":"true","id":"01HJ9NZT45B03C9J6YJJJDB8PG","interactive":"true","name":"server-dev"}
go run . server --dev --address 127.0.0.1:9999 --tls /tmp/runme/tls 2>&1
```

### Upgrading Minor Version Dependencies

For upgrading dependencies that have minor version changes, as well as test dependencies, use the following command. This command fetches the latest minor or patch versions of the modules required for building the current module, including their dependencies:
Periodically dependencies need to be upgraded. For minor versions with test deps:

```sh {"id":"01HF7BT3HEQBTBM9SSSG798S2D"}
$ go get -t -u ./...
```

### Upgrading Major Version Dependencies

When you need to upgrade to major versions of your dependencies, it’s prudent to upgrade each dependency one at a time to manage potential breaking changes efficiently. The `gomajor` tool can assist you in listing and upgrading major version dependencies. To list all major version upgrades available for your project, use the following command:

```sh {"id":"01HF7BT3HEQBTBM9SSSK6JMS58"}
$ gomajor list
```

### Coverage

In order to generate a coverage report, run tests using

```sh {"id":"01J5P9MKFZ4SRS1VE6JCT00N4K"}
make test-coverage
```

And then, for the html coverage report, run:

```sh {"id":"01HJVHEVPX2AZJ86999P1MY5H0"}
make test/coverage/html
```

Output coverage profile information for each function:

```sh {"id":"01HJVHHNMZRNK0ZGA154A9AJCZ"}
make test/coverage/func
```

## Generated Files

### Protocol Buffers

To generate protocol buffers:

```sh {"id":"01HF7BT3HEQBTBM9SSTSGD14KY","interactive":"false"}
make proto/generate
```

Protocol buffer generation is done with [buf](https://buf.build/), and the buf CLI will need to be installed in order for `make proto/generate` to work.

Currently, we use `timostamm-protobuf-ts` to generate TypeScript definitions. These are uploaded to [a remote `buf.build` registry](https://buf.build/gen/npm/v1/@buf/stateful_runme.community_timostamm-protobuf-ts) which can be used in any `npm` compatible project - see the [client-ts](/examples/client-ts/) example.

Note that for protocol buffers to work with `npm` projects, you'll need to add the [`@buf` registry](https://docs.buf.build/bsr/remote-packages/npm) to your `npm` or `yarn` config:

```sh {"id":"01HF7BT3HEQBTBM9SSTSY4P9YW","interactive":"true"}
npm config set @buf:registry https://buf.build/gen/npm/v1
```

```sh {"id":"01HF7BT3HEQBTBM9SSTTB4NANA"}
yarn config set @buf:registry https://buf.build/gen/npm/v1
```

### Enable Runme Extension Development

While development it's not prudent to publish to the Buf's BSR (NPM etc). Instead, you can overwrite the generated types locally. Generate buffers:

```sh {"id":"01HF7BT3HEQBTBM9SSTX2S878Z"}
make proto/generate
```

Then overwrite buffers in the Runme extension's development project. Make sure to delete superfluous TS files to prevent bundler from stumbling.

```sh {"id":"01HF7BT3HEQBTBM9SSTYEHXXYE","name":"proto-dev"}
export RUNME_EXT_BASE="../vscode-runme"
make proto/dev
```

Optionally, reset Runme extension's development project to NPM distributed buffers.

```sh {"id":"01HF7BT3HEQBTBM9SSV0J14G58","name":"proto-dev-reset"}
export RUNME_EXT_BASE="../vscode-runme"
make proto/dev/reset
```

### Publish Protobuf Definitions

Firstly, set up your Buffer (Buf) token by exporting it as an environment variable. Replace `Your buf token` with your actual Buf token:

```sh {"id":"01HF7BT3HEQBTBM9SSSNM5ZT19"}
export BUF_TOKEN=Your buf token
```

After setting the Buf token, proceed to publish the updated protobuf definitions. The `make proto/publish` command below will trigger the publishing process, ensuring your protobuf definitions are released and available for use:

```sh {"id":"01HF7BT3HEQBTBM9SSSPD5B5WW","name":"proto-publish-buf"}
make proto/publish
```

### GraphQL

GraphQL schema are generated as part of:

```sh {"id":"01HF7BT3HEQBTBM9SSV36518MW","interactive":"false"}
make generate
```

This project uses [genqlient](https://github.com/Khan/genqlient) to generate a GraphQL client for interacting with the Stateful API. You will need to install `genqlient` in order for `go generate` to work properly:

```sh {"id":"01HF7BT3HEQBTBM9SSV4MQT4AF","interactive":"false"}
go install github.com/Khan/genqlient
```

See also [the README](/internal/client/graphql/schema/README.md) for generating GraphQL schema from the remote endpoint, which is a pre-requisite for running `go generate`.

### Mocks

Mocks are generated as part of:

```sh {"id":"01HF7BT3HEQBTBM9SSV8D8K14G","interactive":"false"}
make generate
```

This project uses [gomock](https://github.com/golang/mock) to generate some mocks for some interfaces. You will need to install `gomock` in order for `go generate` to work.

## Release Kernel/CLI

The releaser uses `goreleaser` to handle cross-compilation, as well as snapshotting etc. This is run with:

```sh {"id":"01HF7BT3HF9WY615MNGP2PTHHR","interactive":"false"}
make release
```

The requisite tools can be installed with:

```sh {"id":"01HF7BT3HF9WY615MNGR8HJEKZ","interactive":"false"}
make install/goreleaser
```

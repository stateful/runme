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
- Create a topic branch from where you want to base your work. This is usually master.
- Open a new pull request, label it `work in progress` and outline what you will be contributing
- Make commits of logical units.
- Make sure you sign-off on your commits `git commit -s -m "adding X to change Y"`
- Write good commit messages (see below).
- Push your changes to a topic branch in your fork of the repository.
- As you push your changes, update the pull request with new infomation and tasks as you complete them
- Project maintainers might comment on your work as you progress
- When you are done, remove the `work in progess` label and ping the maintainers for a review

## Prerequisites

This project uses a `Makefile` to manage build scripts. You will need `make` installed to run these scripts. See [Makefile](/Makefile) for a list of possible commands and what they do.

You will need to have a `go` installation - ideally compatible with the project's current go version (see [go.mod](/go.mod)).

To install required CLI tools for development, use the `make install/dev` command.

## Build Overview

This codebase is used for a few different purposes, and so there's quite a lot of architecture involved. 

Following is some documentation on the different build targets `runme` has, and what is required to get them to build/run.

### Build Targets

#### CLI

CLI is built with `make build`. This builds the CLI binary for the current platform to an executable file "runme" in the root directory. (You can change the output with the `-o` flag)

#### WASM

WASM is built with `make wasm`. This builds the wasm file to `examples/web/runme.wasm`.

### Generated Files

#### Protocol Buffers

To generate protocol buffers, run `make proto/generate`.

Protocol buffer generation is done with [buf](https://buf.build/), and the buf CLI will need to be installed in order for `make proto/generate` to work.

Currently, we use `timostamm-protobuf-ts` to generate TypeScript definitions. These are uploaded to [a remote `buf.build` registry][registry] which can be used in any `npm` compatible project - see the [client-ts](/examples/client-ts/) example.

Note that for protocol buffers to work with `npm` projects, you'll need to add the [`@buf` registry](https://docs.buf.build/bsr/remote-packages/npm) to your `npm` or `yarn` config.

[registry]: https://buf.build/gen/npm/v1/@buf/stateful_runme.community_timostamm-protobuf-ts

#### GraphQL

GraphQL schema are generated as part of `make generate` (`go generate`).

This project uses [genqlient](https://github.com/Khan/genqlient) to generate a GraphQL client for interacting with the Stateful API. You will need to install `genqlient` in order for `go generate` to work properly.

See also [the README](/internal/client/graphql/schema/README.md) for generating GraphQL schema from the remote endpoint, which is a pre-requisite for running `go generate`.

#### Mocks

Mocks are generated as part of `make generate` (`go generate`).

This project uses [gomock](https://github.com/golang/mock) to generate some mocks for some interfaces. You will need to install `gomock` in order for `go generate` to work.

## Releasing

The releaser uses `goreleaser` to handle cross-compilation, as well as snapshotting etc. This is run with the `make release` command, and the requisite tools can be installed with `make install/goreleaser`.

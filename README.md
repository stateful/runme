[![Runme](./.github/images/github-header.png)](https://runme.dev)

# Runme [![ci](https://github.com/stateful/runme/actions/workflows/ci.yml/badge.svg)](https://github.com/stateful/runme/actions/workflows/ci.yml) [![Join us on Discord](https://img.shields.io/discord/878764303052865537?color=5b39df&label=Join%20us%20on%20Discord)](https://discord.com/invite/BQm8zRCBUY)

> Discover and run code snippets directly from your markdown files, e.g. runbooks, docs, or READMEs (defaults to local `README.md`).

[Runme](https://runme.dev) is a tool that makes runbooks actually runnable, making it easier to follow step-by-step instructions. Users can execute instructions, check intermediate results, and ensure the desired outputs are achieved. Authors can create predefined golden paths and share them with others. Runme combines the guardrails of a pipeline with the flexibility of scripting, where users can check intermediary results before moving on.

Runme achieves this by literally running markdown (ubiquitous for docs inside repos). More specifically, Runme runs your commands inside your fenced code blocks (shell, bash, zsh). It's 100% compatible with your programming language's task definitions (Makefile, Gradle, Grunt, NPM scripts, Pipfile or Deno tasks, etc.). Runme persists your runbooks in markdown, which your docs are likely already using.

<p align="center">
  <img src="./.github/images/hello-world.gif" />
</p>

## Installation

The easiest way on MacOS is to use Homebrew:

```sh { name=update-brew }
$ brew update
```

Install runme:

```sh { name=install-runme }
$ brew install runme
```

or via NPM:

```sh { name=install-npm }
$ npm install -g runme
```

Alternatively, check out [Runme's releases](https://github.com/stateful/runme/releases) and select
a binary for your operating system.

If you have Go developer tools installed, you can install it with `go install`:

```sh { name=install-via-go }
$ go install github.com/stateful/runme@latest
```

## Commands

The Runme CLI contains several commands that allow you to discover and run workflows within your project.

### Run Workflows

Given the following `README.md` file:

````md
# My Project

## Install

First update Brew dependencies:

```sh { name=update-brew }
brew update
```

...
`````

You can run this code cell by just calling

```sh
$ runme run update-brew
```

Read more about how you can configure code cells in the [Runme documentation](https://docs.runme.dev/configuration).

### List

Explore which workflows are available in your project.

```sh { name=runme-list closeTerminalOnSuccess=false interactive=false }
$ runme list
```

### Print

Instead of running the code of a code cell, `print` just outputs the code that would have been executed.

```sh { name=runme-print interactive=false }
$ runme print hello-world
```

### Help

Find help and information to parameters and configurations.

```sh { name=runme-help interactive=false }
$ runme help
```

## Examples

You can find an exhaustive list of examples in the [official Runme documentation](https://runme.dev/examples) with examples demonstrating various use cases of Runme.

## Feedback

Let us know what you think via [GitHub issues](https://github.com/stateful/runme/issues/new) or submit a PR. Join the conversation [on Discord](https://discord.gg/runme). We're looking forward to hear from you.

## Community

The Runme community is growing, join us!

- Ask questions and be curious with us [on Discord](https://discord.gg/runme)
- Read about real live Runme examples and use cases in [our blog](https://runme.dev/blog)
- Meet the developers, learn about the roadmap and recent developments in our [Community Meetings](https://runme.dev/events)
- Subscribe for updates to [our newsletter](https://runme.dev/list)

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md) for more information or just click on:

[![Run with Runme](https://badgen.net/badge/Run%20with/Runme/5B3ADF?icon=https://runme.dev/img/logo.svg)](https://runme.dev/api/runme?repository=https%3A%2F%2Fgithub.com%2Fstateful%2Frunme.git&fileToOpen=CONTRIBUTING.md)

## LICENCE

Apache License, Version 2.0

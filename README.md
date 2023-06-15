[![Runme](./.github/images/github-header.png)](https://runme.dev)

# Runme [![ci](https://github.com/stateful/runme/actions/workflows/ci.yml/badge.svg)](https://github.com/stateful/runme/actions/workflows/ci.yml) [![Join us on Discord](https://img.shields.io/discord/878764303052865537?color=5b39df&label=Join%20us%20on%20Discord)](https://discord.com/invite/BQm8zRCBUY)

> Discover and run code snippets directly from your markdown files, e.g. runbooks, docs, or READMEs (defaults to local `README.md`).

Runme bridges the gap between workflow documentation and task definitions required to develop locally and execute runbooks. It allows project contributors to execute instructions step-by-step, checking intermediary results as they go, to ultimately complete and verify the desired results.

<p align="center">
  <img src="https://raw.githubusercontent.com/stateful/runme.dev/main/static/img/runme-tui.gif" />
</p>

## Installation

The easiest way on MacOS is to use Homebrew:

```sh { name=update-brew }
$ brew update
```

Install runme:

```sh { name=install-runme }
$ brew install stateful/tap/runme
```

or via NPM:

```sh { name=install-npm }
$ npm install -g runme
```

Alternatively, check out [runme's releases](https://github.com/stateful/runme/releases) and select
a binary for your operating system.

If you have Go developer tools installed, you can install it with `go install`:

```sh { name=install-via-go }
$ go install github.com/stateful/runme@latest
```

## Commands

### Run Workflows

```sh { name=runme-run }
$ runme run update-brew
```

### List

```sh { name=runme-list closeTerminalOnSuccess=false interactive=false }
$ runme list
```

### Print

```sh { name=runme-print interactive=false }
$ runme print hello-world
```

### Help

```sh { name=runme-help interactive=false }
$ runme help
```

## Feedback

Let us know what you think via GitHub issues or submit a PR. Join the conversation [on Discord](https://discord.gg/MFtwcSvJsk). We're looking forward to hear from you.

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md) for more information or just click on:

[![Run with Runme](https://badgen.net/badge/Run%20with/Runme/5B3ADF?icon=https://runme.dev/img/logo.svg)](https://runme.dev/api/runme?repository=https%3A%2F%2Fgithub.com%2Fstateful%2Frunme.git&fileToOpen=CONTRIBUTING.md)


## LICENCE

Apache License, Version 2.0

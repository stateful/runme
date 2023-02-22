# runme - run your README.md (markdown docs)

Discover and run code snippets directly from your `README.md` or other markdowns (defaults to local `README.md`).

[![](https://badgen.net/badge/Run%20this%20/README/5B3ADF?icon=https://runme.dev/img/logo.svg)](https://runme.dev/api/runme?repository=git%40github.com%3Astateful%2Frunme.git)

<p align="center">
  <img src="https://user-images.githubusercontent.com/16108792/219203990-ffb860e7-5314-4a22-bf05-9d983e3876d0.gif" />
</p>

runme makes a best effort approach to extracts all code snippets defined in code blocks and allowing to explore and execute them. runme is currently in early alpha.

You can execute commands from a different directory using a `--chdir` flag.
To select a different file than `README.md`, use `--filename`.

## Installation

The easiest way on MacOS is to use Homebrew:

```sh { name=update-brew }
$ brew update
```

Install runme:

```sh { name=install-runme }
$ brew install stateful/tap/runme
```

Alternatively, check out [runme's releases](https://github.com/stateful/runme/releases) and select
a binary for your operating system.

If you have Go developer tools installed, you can install it with `go install`:

```sh { name=install-via-go }
$ go install github.com/stateful/runme@latest
```

## Commands

### Help

```sh { name=runme-help interactive=false }
$ runme help
```

### List

```sh { name=runme-list closeTerminalOnSuccess=false interactive=false }
$ runme list
```

### Print

```sh { name=runme-print interactive=false }
$ runme print hello-world
```

### Run selected command, Example: Update brew

```sh { name=runme-run }
$ runme run update-brew
```

### Example Command

```sh { name=hello-world closeTerminalOnSuccess=false interactive=false }
echo "hello world"
```

## Contributing & Feedback

Let us know what you think via GitHub issues or submit a PR. Join the conversation [on Discord](https://discord.gg/MFtwcSvJsk). We're looking forward to hear from you.

## LICENCE

Apache License, Version 2.0

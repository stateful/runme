# runme

Discover and run code snippets directly from your `README.md` or other markdowns (defaults to local `README.md`).

runme makes a best effort approach to extracts all code snippets defined in code blocks and allowing to explore and execute them. runme is currently in early alpha.

You can execute commands from a different directory using a `--chdir` flag.
To select a different file than `README.md`, use `--filename`.

## Installation

The easiest way on MacOS is to use Homebrew:

```sh {"id":"01HFW6VKQX9B4ZJH9TFYET10X4"}
$ brew install stateful/tap/runme
```

Alternatively, check out [runme's releases](https://github.com/stateful/runme/releases) and select
a binary for your operating system.

If you have Go developer tools installed, you can install it with `go install`:

```sh {"id":"01HFW6VKQX9B4ZJH9TFYH7VEPJ"}
$ go install github.com/stateful/runme@latest
```

## Contributing & Feedback

Let us know what you think via GitHub issues or submit a PR. Join the conversation [on Discord](https://discord.gg/MFtwcSvJsk). We're looking forward to hear from you.

## LICENCE

Apache License, Version 2.0
